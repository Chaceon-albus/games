import { ref, reactive, watch, onMounted } from 'vue'
import type { Player, Room, ChatMessage, Move } from '../types'

export function useGameState() {
  // --- USER PROFILE & NAVIGATION ---
  const nickname = ref('')
  const currentView = ref<'login' | 'lobby' | 'room'>('login')

  onMounted(() => {
    const saved = localStorage.getItem('gomoku_nickname')
    if (saved) {
      nickname.value = saved
      currentView.value = 'lobby'
    } else {
      const randNum = Math.floor(100 + Math.random() * 900)
      nickname.value = `五子棋玩家#${randNum}`
    }
  })

  const saveNickname = (name: string) => {
    if (!name.trim()) return
    nickname.value = name.trim()
    localStorage.setItem('gomoku_nickname', nickname.value)
    currentView.value = 'lobby'
  }

  // --- LOBBY ROOMS STATE ---
  const rooms = reactive<Room[]>([
    {
      id: '1',
      name: '桃源仙境 (初级对弈)',
      status: 'waiting',
      playerCount: 1,
      maxPlayers: 2,
      creatorName: '棋圣小张'
    },
    {
      id: '2',
      name: '竹林小轩 (切磋棋艺)',
      status: 'playing',
      playerCount: 2,
      maxPlayers: 2,
      creatorName: '五子豪杰'
    },
    {
      id: '3',
      name: '水晶王座 (高手对决)',
      status: 'full',
      playerCount: 2,
      maxPlayers: 2,
      creatorName: '智勇双全'
    },
    {
      id: '4',
      name: '棋仙洞府 (观战专区)',
      status: 'playing',
      playerCount: 2,
      maxPlayers: 2,
      creatorName: '世外高人'
    }
  ])

  const activeRoom = ref<Room | null>(null)

  // --- SIMULATION ROLE FOR TESTING ---
  const simulationRole = ref<'black' | 'white' | 'spectator'>('black')

  // --- ACTIVE ROOM GAMEPLAY STATE ---
  const boardSize = 15
  const board = ref<(null | 'black' | 'white')[][]>(
    Array(boardSize)
      .fill(null)
      .map(() => Array(boardSize).fill(null))
  )
  const history = ref<Move[]>([])
  const turn = ref<'black' | 'white'>('black')
  const gameStatus = ref<'idle' | 'ready' | 'playing' | 'ended'>('idle')
  const winner = ref<'black' | 'white' | null>(null)
  const winningLine = ref<{ row: number; col: number }[]>([])

  // Room Players (Mocked for multiplayer feeling)
  const playerBlack = ref<Player | null>(null)
  const playerWhite = ref<Player | null>(null)
  const spectators = ref<Player[]>([])

  // Chat Feed
  const chatMessages = ref<ChatMessage[]>([])

  // --- CORE SYSTEM HOOKS (WebSocket-Ready) ---

  const joinRoom = (room: Room) => {
    activeRoom.value = room
    currentView.value = 'room'
    resetGameState()

    chatMessages.value = [
      {
        id: 'sys1',
        senderName: '系统',
        text: `欢迎来到房间【${room.name}】！`,
        timestamp: getFormattedTime(),
        isSystem: true
      },
      {
        id: 'sys2',
        senderName: '系统',
        text: '提示：您可以点击顶部切换身份来测试对战或观战特性。',
        timestamp: getFormattedTime(),
        isSystem: true
      }
    ]
  }

  const createRoom = (roomName: string) => {
    if (!roomName.trim()) return
    const newRoom: Room = {
      id: String(rooms.length + 1),
      name: roomName.trim(),
      status: 'waiting',
      playerCount: 1,
      maxPlayers: 2,
      creatorName: nickname.value
    }
    rooms.unshift(newRoom)
    joinRoom(newRoom)
  }

  const leaveRoom = () => {
    activeRoom.value = null
    currentView.value = 'lobby'
    resetGameState()
  }

  const resetGameState = () => {
    board.value = Array(boardSize)
      .fill(null)
      .map(() => Array(boardSize).fill(null))
    history.value = []
    turn.value = 'black'
    gameStatus.value = 'idle'
    winner.value = null
    winningLine.value = []

    if (!activeRoom.value) return

    if (simulationRole.value === 'black') {
      playerBlack.value = {
        id: 'user',
        name: nickname.value,
        role: 'player',
        color: 'black',
        isReady: false
      }
      playerWhite.value = {
        id: 'mock_white',
        name: '水晶棋灵 (AI)',
        role: 'player',
        color: 'white',
        isReady: true
      }
      spectators.value = [
        { id: 'spec1', name: '观棋居士', role: 'spectator', color: null, isReady: false },
        { id: 'spec2', name: '吃瓜群众', role: 'spectator', color: null, isReady: false }
      ]
    } else if (simulationRole.value === 'white') {
      playerBlack.value = {
        id: 'mock_black',
        name: '九段高手',
        role: 'player',
        color: 'black',
        isReady: true
      }
      playerWhite.value = {
        id: 'user',
        name: nickname.value,
        role: 'player',
        color: 'white',
        isReady: false
      }
      spectators.value = [
        { id: 'spec1', name: '旁观法师', role: 'spectator', color: null, isReady: false }
      ]
    } else {
      playerBlack.value = {
        id: 'mock_black',
        name: '九段高手',
        role: 'player',
        color: 'black',
        isReady: true
      }
      playerWhite.value = {
        id: 'mock_white',
        name: '水晶棋灵 (AI)',
        role: 'player',
        color: 'white',
        isReady: true
      }
      spectators.value = [
        { id: 'user', name: nickname.value, role: 'spectator', color: null, isReady: false },
        { id: 'spec2', name: '吃瓜群众', role: 'spectator', color: null, isReady: false }
      ]
      gameStatus.value = 'playing'
    }
  }

  // Monitor role switching to update mocked players
  watch(simulationRole, () => {
    resetGameState()
    pushSystemMessage(
      `您已成功切换身份为：${simulationRole.value === 'black' ? '执黑子玩家' : simulationRole.value === 'white' ? '执白子玩家' : '观战者（禁言）'}`
    )
  })

  const toggleReady = () => {
    if (simulationRole.value === 'spectator') return

    if (simulationRole.value === 'black' && playerBlack.value) {
      playerBlack.value.isReady = !playerBlack.value.isReady
      pushSystemMessage(
        `玩家【${playerBlack.value.name}】已${playerBlack.value.isReady ? '准备' : '取消准备'}`
      )
    } else if (simulationRole.value === 'white' && playerWhite.value) {
      playerWhite.value.isReady = !playerWhite.value.isReady
      pushSystemMessage(
        `玩家【${playerWhite.value.name}】已${playerWhite.value.isReady ? '准备' : '取消准备'}`
      )
    }

    if (playerBlack.value?.isReady && playerWhite.value?.isReady) {
      gameStatus.value = 'playing'
      pushSystemMessage('双方准备完毕，对局正式开始！执黑子（先手）落子。')
    }
  }

  // --- GAMEPLAY LOGIC ---

  const placeStone = (row: number, col: number) => {
    if (gameStatus.value !== 'playing') return
    if (simulationRole.value === 'spectator') return
    if (board.value[row][col] !== null) return

    const currentPlayerColor = turn.value
    board.value[row][col] = currentPlayerColor
    history.value.push({ row, col, player: currentPlayerColor })

    const winCoords = checkWin(row, col, currentPlayerColor)
    if (winCoords) {
      gameStatus.value = 'ended'
      winner.value = currentPlayerColor
      winningLine.value = winCoords

      const winnerName =
        currentPlayerColor === 'black'
          ? playerBlack.value?.name || '黑方'
          : playerWhite.value?.name || '白方'
      pushSystemMessage(`恭喜！【${winnerName}】达成五子相连，获得本局胜利！🎉`)
      return
    }

    if (history.value.length === boardSize * boardSize) {
      gameStatus.value = 'ended'
      pushSystemMessage('棋盘已满！本局对决以平局结束。🤝')
      return
    }

    turn.value = turn.value === 'black' ? 'white' : 'black'
  }

  const retractMove = () => {
    if (simulationRole.value === 'spectator') return
    if (history.value.length === 0) return

    const lastMove = history.value.pop()!
    board.value[lastMove.row][lastMove.col] = null
    turn.value = lastMove.player

    if (gameStatus.value === 'ended') {
      gameStatus.value = 'playing'
      winner.value = null
      winningLine.value = []
      pushSystemMessage('已执行悔棋，对局继续！')
    } else {
      pushSystemMessage(`已悔棋，轮到【${turn.value === 'black' ? '黑方' : '白方'}】落子。`)
    }
  }

  const resignGame = () => {
    if (gameStatus.value !== 'playing') return
    if (simulationRole.value === 'spectator') return

    const resigningPlayer = simulationRole.value
    const winningPlayerColor = resigningPlayer === 'black' ? 'white' : 'black'

    gameStatus.value = 'ended'
    winner.value = winningPlayerColor

    const resigningName =
      resigningPlayer === 'black' ? playerBlack.value?.name : playerWhite.value?.name
    const winningName =
      winningPlayerColor === 'black' ? playerBlack.value?.name : playerWhite.value?.name

    pushSystemMessage(`【${resigningName}】认输。恭喜【${winningName}】获得本局胜利！🏅`)
  }

  // --- CHAT LOGIC ---

  const sendChat = (text: string) => {
    if (!text.trim()) return
    if (simulationRole.value === 'spectator') return

    chatMessages.value.push({
      id: String(Date.now()),
      senderName: nickname.value,
      text: text.trim(),
      timestamp: getFormattedTime()
    })

    simulateMockChatResponse(text.trim())
  }

  const pushSystemMessage = (text: string) => {
    chatMessages.value.push({
      id: String(Date.now()),
      senderName: '系统',
      text,
      timestamp: getFormattedTime(),
      isSystem: true
    })
  }

  // --- HELPER ALGORITHMS ---

  const getFormattedTime = () => {
    const d = new Date()
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }

  const checkWin = (r: number, c: number, color: 'black' | 'white') => {
    const directions = [
      [
        [0, 1],
        [0, -1]
      ], // Horizontal
      [
        [1, 0],
        [-1, 0]
      ], // Vertical
      [
        [1, 1],
        [-1, -1]
      ], // Diagonal \
      [
        [1, -1],
        [-1, 1]
      ] // Counter-diagonal /
    ]

    for (const dir of directions) {
      const coords = [{ row: r, col: c }]
      for (const [dr, dc] of dir) {
        let nr = r + dr
        let nc = c + dc
        while (
          nr >= 0 &&
          nr < boardSize &&
          nc >= 0 &&
          nc < boardSize &&
          board.value[nr][nc] === color
        ) {
          coords.push({ row: nr, col: nc })
          nr += dr
          nc += dc
        }
      }
      if (coords.length >= 5) {
        return coords.sort((a, b) => a.row - b.row || a.col - b.col)
      }
    }
    return null
  }

  const simulateMockChatResponse = (_playerMsg: string) => {
    setTimeout(() => {
      if (activeRoom.value && gameStatus.value === 'playing') {
        const responses = [
          '好棋！这步下得妙啊。',
          '哎呀，我刚才大意了。',
          '局势焦灼，阁下内力深厚！',
          '哈哈，来决一胜负吧！',
          '容我三思……',
          '承让承让，阁下棋路开阔。'
        ]
        const randomMsg = responses[Math.floor(Math.random() * responses.length)]
        const responder =
          simulationRole.value === 'black' ? playerWhite.value?.name : playerBlack.value?.name
        chatMessages.value.push({
          id: String(Date.now()),
          senderName: responder || '对手',
          text: randomMsg,
          timestamp: getFormattedTime()
        })
      }
    }, 1500)
  }

  return {
    nickname,
    currentView,
    rooms,
    activeRoom,
    simulationRole,
    board,
    boardSize,
    history,
    turn,
    gameStatus,
    winner,
    winningLine,
    playerBlack,
    playerWhite,
    spectators,
    chatMessages,
    saveNickname,
    createRoom,
    joinRoom,
    leaveRoom,
    toggleReady,
    placeStone,
    retractMove,
    resignGame,
    sendChat,
    resetGameState
  }
}
