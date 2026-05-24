export interface Player {
  id: string
  name: string
  role: 'player' | 'spectator'
  color: 'black' | 'white' | null
  isReady: boolean
  isOffline?: boolean
}

export interface Room {
  id: string
  name: string
  status: 'waiting' | 'playing' | 'full'
  playerCount: number
  maxPlayers: number
  creatorName: string
}

export interface ChatMessage {
  id: string
  senderName: string
  text: string
  timestamp: string
  isSystem?: boolean
}

export interface Move {
  row: number
  col: number
  player: 'black' | 'white'
}
