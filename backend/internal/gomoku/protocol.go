package gomoku

import "encoding/json"

const (
	MaxRooms             = 256
	MaxSpectatorsPerRoom = 256
	MaxChatLength        = 100
)

const (
	ErrorInvalidSession    = "INVALID_SESSION"
	ErrorInvalidPayload    = "INVALID_PAYLOAD"
	ErrorAlreadyInRoom     = "ALREADY_IN_ROOM"
	ErrorRoomNotFound      = "ROOM_NOT_FOUND"
	ErrorRoomCapacity      = "ROOM_CAPACITY_REACHED"
	ErrorRoomLimit         = "ROOM_LIMIT_REACHED"
	ErrorNotRoomMember     = "NOT_ROOM_MEMBER"
	ErrorPermissionDenied  = "PERMISSION_DENIED"
	ErrorRateLimited       = "RATE_LIMITED"
	ErrorChatDisabled      = "CHAT_DISABLED"
	ErrorSpectatorChat     = "SPECTATOR_CHAT_FORBIDDEN"
	ErrorInvalidRoomConfig = "INVALID_ROOM_CONFIG"
)

type ClientMessage struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type CreateRoomPayload struct {
	Name   string     `json:"name"`
	Config RoomConfig `json:"config"`
}

type JoinRoomPayload struct {
	RoomID string `json:"roomId"`
}

type PlaceStonePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type RetractRespondPayload struct {
	Agree bool `json:"agree"`
}

type SendChatPayload struct {
	Text string `json:"text"`
}

type ConfigureRoomPayload struct {
	Config RoomConfig `json:"config"`
}

type ServerMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PlayerView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	IsReady   bool   `json:"isReady"`
	IsOffline bool   `json:"isOffline"`
}

type SelfView struct {
	PlayerID string `json:"playerId"`
	Role     string `json:"role"`
	Color    string `json:"color,omitempty"`
}

type RoomStateView struct {
	ID                     string       `json:"id"`
	Name                   string       `json:"name"`
	Status                 string       `json:"status"`
	Host                   *PlayerView  `json:"host"`
	Opponent               *PlayerView  `json:"opponent"`
	Spectators             []PlayerView `json:"spectators"`
	Config                 RoomConfig   `json:"config"`
	History                []Move       `json:"history"`
	Turn                   string       `json:"turn"`
	Winner                 string       `json:"winner"`
	WinningLine            []Coord      `json:"winningLine"`
	HostColor              string       `json:"hostColor"`
	OpponentColor          string       `json:"opponentColor"`
	ConsecutiveGames       int          `json:"consecutiveGames"`
	RetractPending         bool         `json:"retractPending"`
	RetractRequesterName   string       `json:"retractRequesterName"`
	RetractRequestedBySelf bool         `json:"retractRequestedBySelf"`
	Self                   SelfView     `json:"self"`
}

type LobbyRoom struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	PlayerCount    int    `json:"playerCount"`
	MaxPlayers     int    `json:"maxPlayers"`
	SpectatorCount int    `json:"spectatorCount"`
	MaxSpectators  int    `json:"maxSpectators"`
	CreatorName    string `json:"creatorName"`
}

type ChatMessage struct {
	ID         string `json:"id"`
	SenderID   string `json:"senderId,omitempty"`
	SenderName string `json:"senderName"`
	Text       string `json:"text"`
	Timestamp  string `json:"timestamp"`
	IsSystem   bool   `json:"isSystem,omitempty"`
}
