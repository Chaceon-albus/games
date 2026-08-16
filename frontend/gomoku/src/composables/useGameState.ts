import { ref, reactive, watch, onMounted, computed, onUnmounted } from 'vue'
import type {
  ActiveRoom,
  ChatMessage,
  ErrorPayload,
  GameWinner,
  Move,
  Player,
  Room,
  RoomConfig,
  StoneColor
} from '../types'

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
  const playerId = ref('')
  const currentView = ref<'login' | 'lobby' | 'room'>('login')
  const isConnected = ref(false)

  // Lobby Rooms
  const rooms = reactive<Room[]>([])
  const activeRoom = ref<ActiveRoom | null>(null)
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
  const turn = ref<StoneColor>('black')
  const gameStatus = ref<'idle' | 'ready' | 'playing' | 'ended'>('idle')
  const winner = ref<GameWinner | null>(null)
  const winningLine = ref<{ row: number; col: number }[]>([])

  // Player Slots
  const playerBlack = ref<Player | null>(null)
  const playerWhite = ref<Player | null>(null)
  const spectators = ref<Player[]>([])

  // Chat Feed
  const chatMessages = ref<ChatMessage[]>([])

  // Connection Ref
  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectDelay = 1000
  let shouldReconnect = true
  let socketSerial = 0

  // Simulation Role Ref for backwards compatibility with UI switcher
  const simulationRole = ref<'black' | 'white' | 'spectator'>('spectator')
  const retractCooldown = ref(0)

  // --- REST AUTHENTICATION ---
  onMounted(async () => {
    const savedUUID = localStorage.getItem('gomoku_uuid')
    const savedName = localStorage.getItem('gomoku_nickname')
    const savedPlayerId = localStorage.getItem('gomoku_player_id')
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
          playerId.value = data.playerId
          localStorage.setItem('gomoku_player_id', data.playerId)
          currentView.value = 'lobby'
          connectWebSocket(savedUUID)
        } else {
          clearLocalStorage()
        }
      } catch (e) {
        // Network offline fallback
        nickname.value = savedName
        playerId.value = savedPlayerId || ''
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
        playerId.value = data.playerId
        localStorage.setItem('gomoku_uuid', data.uuid)
        localStorage.setItem('gomoku_nickname', data.nickname)
        localStorage.setItem('gomoku_player_id', data.playerId)
        currentView.value = 'lobby'
        shouldReconnect = true
        connectWebSocket(data.uuid)
      }
    } catch (e) {
      alert('注册玩家失败，请检查服务器连接')
    }
  }

  const logout = () => {
    shouldReconnect = false
    socketSerial++
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    clearLocalStorage()
    nickname.value = `五子棋玩家#${Math.floor(100 + Math.random() * 900)}`
    playerId.value = ''
    currentView.value = 'login'
    activeRoom.value = null
    resetGameStateVariables()
    if (ws) {
      ws.close()
      ws = null
    }
  }

  const clearLocalStorage = () => {
    localStorage.removeItem('gomoku_uuid')
    localStorage.removeItem('gomoku_nickname')
    localStorage.removeItem('gomoku_player_id')
  }

  // --- WEBSOCKET CLIENT ---
  const connectWebSocket = (uuid: string) => {
    if (ws) return

    const serial = ++socketSerial
    const socket = new WebSocket(`${baseWsUrl}/ws/gomoku?uuid=${uuid}`)
    ws = socket

    socket.onopen = () => {
      if (serial !== socketSerial || ws !== socket) return
      isConnected.value = true
      reconnectDelay = 1000
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }

      // Clear any disconnect warnings
      chatMessages.value = chatMessages.value.filter(msg => !msg.text.includes('连接已断开'))
    }

    socket.onclose = event => {
      if (serial !== socketSerial || ws !== socket) return
      isConnected.value = false
      ws = null

      if (event.code === 4001 || event.reason === 'invalid_session') {
        invalidateSession()
        return
      }
      if (event.code === 4002 || event.reason === 'session_replaced') {
        shouldReconnect = false
        window.alert('当前账号已在另一个页面建立连接，本页面已停止重连。')
        return
      }

      const savedUUID = localStorage.getItem('gomoku_uuid')
      if (savedUUID && shouldReconnect) {
        pushSystemMessage('连接已断开，正在尝试重新连接...')
        reconnectTimer = setTimeout(() => {
          reconnectTimer = null
          reconnectDelay = Math.min(reconnectDelay * 2, 16000)
          connectWebSocket(savedUUID)
        }, reconnectDelay)
      }
    }

    socket.onerror = () => {
      if (ws === socket) socket.close()
    }

    socket.onmessage = event => {
      if (serial !== socketSerial || ws !== socket) return
      try {
        const msg = JSON.parse(event.data)
        handleServerMessage(msg)
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e)
      }
    }
  }

  const invalidateSession = () => {
    shouldReconnect = false
    socketSerial++
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    clearLocalStorage()
    isConnected.value = false
    playerId.value = ''
    activeRoom.value = null
    currentView.value = 'login'
    resetGameStateVariables()
    const currentSocket = ws
    ws = null
    if (currentSocket) currentSocket.close()
    window.alert('登录状态已失效，请重新进入游戏。')
  }

  const sendWsAction = (action: string, data?: unknown) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ action, data: data || undefined }))
      return true
    }
    return false
  }

  // --- SERVER INCOMING ACTION HANDLER ---
  const handleServerMessage = (msg: { type: string; data: unknown }) => {
    switch (msg.type) {
      case 'room_list':
        rooms.splice(0, rooms.length, ...(msg.data as Room[]))
        if (isManualRefreshing.value) {
          isManualRefreshing.value = false
          manualRefreshCount.value = (msg.data as Room[]).length
        }
        break

      case 'room_state':
        syncRoomState(msg.data as ActiveRoom | null)
        break

      case 'chat_message':
        chatMessages.value.push(msg.data as ChatMessage)
        break

      case 'error_message': {
        const error = msg.data as ErrorPayload
        if (error.code === 'INVALID_SESSION') {
          invalidateSession()
          return
        }
        pushSystemMessage(`⚠️ 错误提示: ${error.message}`)
        break
      }
    }
  }

  // Sync server room state to reactive refs
  const syncRoomState = (room: ActiveRoom | null) => {
    activeRoom.value = room

    if (!room) {
      currentView.value = 'lobby'
      resetGameStateVariables()
      return
    }

    currentView.value = 'room'
    history.value = room.history
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
    winningLine.value = room.winningLine

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
    const activeRole: StoneColor | 'spectator' = room.self.color || 'spectator'

    if (host) {
      const isHostBlack = room.hostColor === 'black'
      const hostPlayer: Player = {
        id: host.id,
        name: host.name,
        avatar: host.avatar,
        role: 'player',
        color: room.hostColor,
        isReady: host.isReady,
        isOffline: host.isOffline
      }
      if (isHostBlack) pBlack = hostPlayer
      else pWhite = hostPlayer
    }

    if (opponent) {
      const isOpponentBlack = room.opponentColor === 'black'
      const opponentPlayer: Player = {
        id: opponent.id,
        name: opponent.name,
        avatar: opponent.avatar,
        role: 'player',
        color: room.opponentColor,
        isReady: opponent.isReady,
        isOffline: opponent.isOffline
      }
      if (isOpponentBlack) pBlack = opponentPlayer
      else pWhite = opponentPlayer
    }

    playerBlack.value = pBlack
    playerWhite.value = pWhite

    spectators.value = room.spectators.map(spec => {
      return {
        id: spec.id,
        name: spec.name,
        avatar: spec.avatar,
        role: 'spectator',
        color: null,
        isReady: false,
        isOffline: spec.isOffline
      }
    })

    // Set simulation role for UI binding compatibility
    simulationRole.value = activeRole
    playerId.value = room.self.playerId
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
    if (!roomName.trim() || !isConnected.value) return false
    sendWsAction('create_room', {
      name: roomName.trim(),
      config: {
        autoJoinSpectator: false,
        disableChat: false,
        colorMode: 'alternating'
      }
    })
    return true
  }

  const joinRoom = (room: Room) => {
    return sendWsAction('join_room', { roomId: room.id })
  }

  const leaveRoom = () => {
    sendWsAction('leave_room')
  }

  const toggleReady = () => {
    return sendWsAction('toggle_ready')
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
    if (!isConnected.value) return
    if (gameStatus.value !== 'playing') return
    if (simulationRole.value === 'spectator') return
    if (board.value[row][col] !== null) return
    if (turn.value !== simulationRole.value) return

    if (sendWsAction('place_stone', { row, col })) {
      playPlaceSound()
    }
  }

  const retractMove = () => {
    if (simulationRole.value === 'spectator') return
    if (activeRoom.value?.retractPending) return
    if (retractCooldown.value > 0) return

    if (!sendWsAction('request_retract')) return

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
    return sendWsAction('resign')
  }

  const sendChat = (text: string) => {
    if (!text.trim()) return false
    return sendWsAction('send_chat', { text: text.trim() })
  }

  // --- NEW MULTIPLAYER UTILS ---

  const respondRetract = (agree: boolean) => {
    sendWsAction('retract_respond', { agree })
  }

  const updateRoomConfig = (config: RoomConfig) => {
    sendWsAction('configure_room', { config })
  }

  const claimSeat = () => {
    sendWsAction('claim_seat')
  }

  const refreshRooms = () => {
    isManualRefreshing.value = true
    if (!sendWsAction('list_rooms')) {
      isManualRefreshing.value = false
    }
  }

  // Computed properties for retract requesting dialogue
  const retractRequesterName = computed(() => activeRoom.value?.retractRequesterName || '')
  const showRetractDialog = computed(() => {
    return activeRoom.value?.retractPending === true && !activeRoom.value.retractRequestedBySelf
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

  onUnmounted(() => {
    shouldReconnect = false
    socketSerial++
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (ws) ws.close()
    ws = null
  })

  return {
    nickname,
    playerId,
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
    claimSeat,
    refreshRooms,
    manualRefreshCount
  }
}
