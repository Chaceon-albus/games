import { ref, reactive, watch, onMounted, computed } from 'vue'
import type { Player, Room, ChatMessage, Move } from '../types'

export function useGameState() {
  // --- ENDPOINTS RESOLVER ---
  const isDev = import.meta.env.DEV
  const host = window.location.hostname || 'localhost'
  const currentPort = window.location.port

  // If already loaded through the Go server (port 8080), use the same origin to prevent CORS/host mismatch.
  // Otherwise, if accessing the Vite dev server directly (e.g. port 5173), target port 8080 on the same host.
  const baseHttpUrl =
    isDev && currentPort !== '8080' ? `http://${host}:8080` : window.location.origin

  const baseWsUrl =
    isDev && currentPort !== '8080'
      ? `ws://${host}:8080`
      : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`

  // --- STATE VARIABLES ---
  const nickname = ref('')
  const currentView = ref<'login' | 'lobby' | 'room'>('login')
  const isConnected = ref(false)

  // Lobby Rooms
  const rooms = reactive<Room[]>([])
  const activeRoom = ref<any | null>(null)
  const isManualRefreshing = ref(false)
  const manualRefreshCount = ref<number | null>(null)

  // Gameplay State (Mapped dynamically from server Room state)
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

  // Player Slots
  const playerBlack = ref<Player | null>(null)
  const playerWhite = ref<Player | null>(null)
  const spectators = ref<Player[]>([])

  // Chat Feed
  const chatMessages = ref<ChatMessage[]>([])

  // Connection Ref
  let ws: WebSocket | null = null
  let reconnectTimer: any = null
  let reconnectDelay = 1000

  // Simulation Role Ref for backwards compatibility with UI switcher
  const simulationRole = ref<'black' | 'white' | 'spectator'>('spectator')
  const retractCooldown = ref(0)

  // --- REST AUTHENTICATION ---
  onMounted(async () => {
    const savedUUID = localStorage.getItem('gomoku_uuid')
    const savedName = localStorage.getItem('gomoku_nickname')
    if (savedUUID && savedName) {
      try {
        const res = await fetch(`${baseHttpUrl}/api/v1/gomoku/verify`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ uuid: savedUUID })
        })
        const data = await res.json()
        if (data.valid) {
          nickname.value = data.nickname
          currentView.value = 'lobby'
          connectWebSocket(savedUUID)
        } else {
          clearLocalStorage()
        }
      } catch (e) {
        // Network offline fallback
        nickname.value = savedName
        currentView.value = 'lobby'
        connectWebSocket(savedUUID)
      }
    } else {
      const randNum = Math.floor(100 + Math.random() * 900)
      nickname.value = `五子棋玩家#${randNum}`
    }
  })

  const saveNickname = async (name: string) => {
    if (!name.trim()) return
    let length = 0
    let truncated = ''
    for (let i = 0; i < name.length; i++) {
      const char = name[i]
      const unit = char.charCodeAt(0) > 127 ? 2 : 1
      if (length + unit > 20) {
        break
      }
      truncated += char
      length += unit
    }
    const finalName = truncated.trim()
    if (!finalName) return

    try {
      const res = await fetch(`${baseHttpUrl}/api/v1/gomoku/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nickname: finalName })
      })
      const data = await res.json()
      if (data.uuid) {
        nickname.value = data.nickname
        localStorage.setItem('gomoku_uuid', data.uuid)
        localStorage.setItem('gomoku_nickname', data.nickname)
        currentView.value = 'lobby'
        connectWebSocket(data.uuid)
      }
    } catch (e) {
      alert('注册玩家失败，请检查服务器连接')
    }
  }

  const logout = () => {
    clearLocalStorage()
    nickname.value = `五子棋玩家#${Math.floor(100 + Math.random() * 900)}`
    currentView.value = 'login'
    if (ws) {
      ws.close()
      ws = null
    }
  }

  const clearLocalStorage = () => {
    localStorage.removeItem('gomoku_uuid')
    localStorage.removeItem('gomoku_nickname')
  }

  // --- WEBSOCKET CLIENT ---
  const pendingActions: Array<() => void> = []

  const connectWebSocket = (uuid: string) => {
    if (ws) return

    ws = new WebSocket(`${baseWsUrl}/ws/gomoku?uuid=${uuid}`)

    ws.onopen = () => {
      isConnected.value = true
      reconnectDelay = 1000
      if (reconnectTimer) clearTimeout(reconnectTimer)

      // Clear any disconnect warnings
      chatMessages.value = chatMessages.value.filter(msg => !msg.text.includes('连接已断开'))

      // Flush pending actions
      while (pendingActions.length > 0) {
        const action = pendingActions.shift()
        if (action) action()
      }
    }

    ws.onclose = () => {
      isConnected.value = false
      ws = null

      const savedUUID = localStorage.getItem('gomoku_uuid')
      if (savedUUID) {
        pushSystemMessage('连接已断开，正在尝试重新连接...')
        reconnectTimer = setTimeout(() => {
          reconnectDelay = Math.min(reconnectDelay * 2, 16000)
          connectWebSocket(savedUUID)
        }, reconnectDelay)
      }
    }

    ws.onerror = () => {
      if (ws) ws.close()
    }

    ws.onmessage = event => {
      try {
        const msg = JSON.parse(event.data)
        handleServerMessage(msg)
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }
  }

  const sendWsAction = (action: string, data?: any) => {
    const send = () => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            action,
            data: data || undefined
          })
        )
      }
    }

    if (ws && ws.readyState === WebSocket.OPEN) {
      send()
    } else {
      pendingActions.push(send)
      const savedUUID = localStorage.getItem('gomoku_uuid')
      if (savedUUID) {
        connectWebSocket(savedUUID)
      }
    }
  }

  // --- SERVER INCOMING ACTION HANDLER ---
  const handleServerMessage = (msg: { type: string; data: any }) => {
    switch (msg.type) {
      case 'room_list':
        rooms.splice(0, rooms.length, ...msg.data)
        if (isManualRefreshing.value) {
          isManualRefreshing.value = false
          manualRefreshCount.value = msg.data.length
        }
        break

      case 'room_state':
        syncRoomState(msg.data)
        break

      case 'chat_message':
        chatMessages.value.push(msg.data)
        break

      case 'error_message':
        pushSystemMessage(`⚠️ 错误提示: ${msg.data.message}`)
        break
    }
  }

  // Sync server room state to reactive refs
  const syncRoomState = (room: any) => {
    activeRoom.value = room

    if (!room) {
      currentView.value = 'lobby'
      resetGameStateVariables()
      return
    }

    currentView.value = 'room'
    history.value = room.history || []
    turn.value = room.turn

    // Map status
    if (room.status === 'playing') {
      gameStatus.value = 'playing'
    } else if (room.status === 'waiting') {
      if (room.winner !== '') {
        gameStatus.value = 'ended'
      } else {
        gameStatus.value = room.host && room.opponent ? 'ready' : 'idle'
      }
    }

    winner.value = room.winner === '' ? null : room.winner
    winningLine.value = room.winningLine || []

    // Rebuild board from history
    const newBoard = Array(boardSize)
      .fill(null)
      .map(() => Array(boardSize).fill(null))
    history.value.forEach(move => {
      newBoard[move.row][move.col] = move.player
    })
    board.value = newBoard

    // Sync Players & Spectators
    const host = room.host
    const opponent = room.opponent

    let pBlack: Player | null = null
    let pWhite: Player | null = null
    let activeRole: 'black' | 'white' | 'spectator' = 'spectator'

    if (host) {
      const isHostBlack = room.hostColor === 'black'
      const hostPlayer: Player = {
        id: host.name === nickname.value ? 'user' : 'host',
        name: host.name,
        role: 'player',
        color: room.hostColor,
        isReady: host.isReady,
        isOffline: host.isOffline
      }
      if (host.name === nickname.value) {
        activeRole = room.hostColor
      }
      if (isHostBlack) pBlack = hostPlayer
      else pWhite = hostPlayer
    }

    if (opponent) {
      const isOpponentBlack = room.opponentColor === 'black'
      const opponentPlayer: Player = {
        id: opponent.name === nickname.value ? 'user' : 'opponent',
        name: opponent.name,
        role: 'player',
        color: room.opponentColor,
        isReady: opponent.isReady,
        isOffline: opponent.isOffline
      }
      if (opponent.name === nickname.value) {
        activeRole = room.opponentColor
      }
      if (isOpponentBlack) pBlack = opponentPlayer
      else pWhite = opponentPlayer
    }

    playerBlack.value = pBlack
    playerWhite.value = pWhite

    spectators.value = (room.spectators || []).map((spec: any) => {
      if (spec.name === nickname.value) {
        activeRole = 'spectator'
      }
      return {
        id: spec.name === nickname.value ? 'user' : 'spectator',
        name: spec.name,
        role: 'spectator',
        color: null,
        isReady: false,
        isOffline: spec.isOffline
      }
    })

    // Set simulation role for UI binding compatibility
    simulationRole.value = activeRole
  }

  const resetGameStateVariables = () => {
    board.value = Array(boardSize)
      .fill(null)
      .map(() => Array(boardSize).fill(null))
    history.value = []
    turn.value = 'black'
    gameStatus.value = 'idle'
    winner.value = null
    winningLine.value = []
    playerBlack.value = null
    playerWhite.value = null
    spectators.value = []
    chatMessages.value = []
  }

  const resetGameState = () => {
    // Left as a no-op / state reset triggers automatically on room state syncs
  }

  // --- ACTIONS SENDERS ---
  const createRoom = (roomName: string) => {
    if (!roomName.trim()) return
    sendWsAction('create_room', {
      name: roomName.trim(),
      config: {
        autoJoinSpectator: false,
        disableChat: false,
        colorMode: 'alternating'
      }
    })
  }

  const joinRoom = (room: Room) => {
    sendWsAction('join_room', { roomId: room.id })
  }

  const leaveRoom = () => {
    sendWsAction('leave_room')
    activeRoom.value = null
    currentView.value = 'lobby'
    resetGameStateVariables()
  }

  const toggleReady = () => {
    sendWsAction('toggle_ready')
  }

  // SOUND EFFECT FOR PLACED STONES
  let audioCtx: AudioContext | null = null
  let noiseBuffer: AudioBuffer | null = null

  const getNoiseBuffer = (ctx: AudioContext): AudioBuffer => {
    if (noiseBuffer) return noiseBuffer
    const sampleRate = ctx.sampleRate
    const length = Math.floor(sampleRate * 0.1)
    const buffer = ctx.createBuffer(1, length, sampleRate)
    const data = buffer.getChannelData(0)
    for (let i = 0; i < length; i++) {
      data[i] = Math.random() * 2 - 1
    }
    noiseBuffer = buffer
    return noiseBuffer
  }

  const playPlaceSound = () => {
    try {
      if (!audioCtx) {
        audioCtx = new AudioContext()
      }
      const ctx = audioCtx
      if (ctx.state === 'suspended') ctx.resume()

      const now = ctx.currentTime
      const masterGain = ctx.createGain()
      masterGain.gain.setValueAtTime(0.5, now)
      masterGain.connect(ctx.destination)

      const randomFactor = Math.random() * 0.06 + 0.97
      const baseFreq = 320 * randomFactor
      const clickVolume = 0.3 * (Math.random() * 0.2 + 0.9)

      const playClick = (timeOffset: number, volume: number, duration: number) => {
        const noiseSource = ctx.createBufferSource()
        noiseSource.buffer = getNoiseBuffer(ctx)

        const filter = ctx.createBiquadFilter()
        filter.type = 'bandpass'
        filter.frequency.setValueAtTime(3200 * randomFactor, now + timeOffset)
        filter.Q.setValueAtTime(1.5, now + timeOffset)

        const gainNode = ctx.createGain()
        gainNode.gain.setValueAtTime(volume, now + timeOffset)
        gainNode.gain.exponentialRampToValueAtTime(0.001, now + timeOffset + duration)

        noiseSource.connect(filter)
        filter.connect(gainNode)
        gainNode.connect(ctx.destination)

        noiseSource.start(now + timeOffset)
        noiseSource.stop(now + timeOffset + duration)
      }

      playClick(0, clickVolume, 0.025)
      playClick(0.012, clickVolume * 0.3, 0.015)

      const resonances = [
        { freq: baseFreq, gain: 0.2, decay: 0.04 },
        { freq: baseFreq * 1.8, gain: 0.12, decay: 0.03 },
        { freq: baseFreq * 2.6, gain: 0.06, decay: 0.02 }
      ]

      resonances.forEach(res => {
        const osc = ctx.createOscillator()
        osc.type = 'sine'
        osc.frequency.setValueAtTime(res.freq, now)

        const gainNode = ctx.createGain()
        gainNode.gain.setValueAtTime(res.gain, now)
        gainNode.gain.exponentialRampToValueAtTime(0.001, now + res.decay)

        osc.connect(gainNode)
        gainNode.connect(masterGain)

        osc.start(now)
        osc.stop(now + res.decay)
      })
    } catch (e) {
      // Fail silently
    }
  }

  // Place stone triggers placement sound instantly, and pushes moves to WS
  const placeStone = (row: number, col: number) => {
    if (gameStatus.value !== 'playing') return
    if (simulationRole.value === 'spectator') return
    if (board.value[row][col] !== null) return
    if (turn.value !== simulationRole.value) return

    // Pre-play place sound for responsive feeling
    playPlaceSound()
    sendWsAction('place_stone', { row, col })
  }

  const retractMove = () => {
    if (simulationRole.value === 'spectator') return
    if (activeRoom.value?.retractRequester) return
    if (retractCooldown.value > 0) return

    sendWsAction('request_retract')

    // Start 5-second cooldown timer
    retractCooldown.value = 5
    const timer = setInterval(() => {
      retractCooldown.value--
      if (retractCooldown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)
  }

  const resignGame = () => {
    if (simulationRole.value === 'spectator') return
    sendWsAction('resign')
  }

  const sendChat = (text: string) => {
    if (!text.trim()) return

    // Add local preview to chat log instantly
    chatMessages.value.push({
      id: String(Date.now()),
      senderName: nickname.value,
      text: text.trim(),
      timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    })

    sendWsAction('send_chat', { text: text.trim() })
  }

  // --- NEW MULTIPLAYER UTILS ---

  const respondRetract = (agree: boolean) => {
    sendWsAction('retract_respond', { agree })
  }

  const updateRoomConfig = (config: {
    autoJoinSpectator: boolean
    disableChat: boolean
    colorMode: string
  }) => {
    sendWsAction('configure_room', { config })
  }

  const refreshRooms = () => {
    isManualRefreshing.value = true
    sendWsAction('list_rooms')
  }

  // Computed properties for retract requesting dialogue
  const retractRequesterName = computed(() => activeRoom.value?.retractRequesterName || '')
  const showRetractDialog = computed(() => {
    return retractRequesterName.value !== '' && retractRequesterName.value !== nickname.value
  })

  // Watch for history changes to trigger sound effect for other player's turns
  watch(
    () => history.value.length,
    (newVal, oldVal) => {
      // Play placement sound when history size increases
      if (newVal > (oldVal || 0)) {
        // Avoid double sounds if placed by self (as self is pre-played)
        const lastMove = history.value[history.value.length - 1]
        const selfColor = simulationRole.value
        if (lastMove && lastMove.player !== selfColor) {
          playPlaceSound()
        }
      }
    }
  )

  const pushSystemMessage = (text: string) => {
    chatMessages.value.push({
      id: `sys_client_${Date.now()}`,
      senderName: '系统',
      text,
      timestamp: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
      isSystem: true
    })
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
    isConnected,
    showRetractDialog,
    retractRequesterName,
    retractCooldown,
    saveNickname,
    createRoom,
    joinRoom,
    leaveRoom,
    toggleReady,
    placeStone,
    retractMove,
    resignGame,
    sendChat,
    resetGameState,
    logout,
    respondRetract,
    updateRoomConfig,
    refreshRooms,
    manualRefreshCount
  }
}
