export type StoneColor = 'black' | 'white'
export type GameWinner = StoneColor | 'draw'
export type RoomRole = 'host' | 'opponent' | 'spectator'

export interface RoomConfig {
  autoJoinSpectator: boolean
  disableChat: boolean
  colorMode: 'alternating' | 'random'
}

export interface PlayerView {
  id: string
  name: string
  avatar: string
  isReady: boolean
  isOffline: boolean
}

export interface Player extends PlayerView {
  role: 'player' | 'spectator'
  color: StoneColor | null
}

export interface SelfView {
  playerId: string
  role: RoomRole
  color?: StoneColor
}

export interface Room {
  id: string
  name: string
  status: 'waiting' | 'playing'
  playerCount: number
  maxPlayers: number
  spectatorCount: number
  maxSpectators: number
  creatorName: string
}

export interface ActiveRoom {
  id: string
  name: string
  status: 'waiting' | 'playing'
  host: PlayerView | null
  opponent: PlayerView | null
  spectators: PlayerView[]
  config: RoomConfig
  history: Move[]
  turn: StoneColor
  winner: GameWinner | ''
  winningLine: Coord[]
  hostColor: StoneColor
  opponentColor: StoneColor
  consecutiveGames: number
  retractPending: boolean
  retractRequesterName: string
  retractRequestedBySelf: boolean
  self: SelfView
}

export interface ChatMessage {
  id: string
  senderId?: string
  senderName: string
  text: string
  timestamp: string
  isSystem?: boolean
}

export interface Move {
  row: number
  col: number
  player: StoneColor
}

export interface Coord {
  row: number
  col: number
}

export interface ErrorPayload {
  code: string
  message: string
}
