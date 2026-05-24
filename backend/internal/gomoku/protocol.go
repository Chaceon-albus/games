package gomoku

import "encoding/json"

// ClientMessage is the container for messages received from a client
type ClientMessage struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// CreateRoomPayload is received when creating a room
type CreateRoomPayload struct {
	Name   string     `json:"name"`
	Config RoomConfig `json:"config"`
}

// JoinRoomPayload is received when joining a room
type JoinRoomPayload struct {
	RoomID string `json:"roomId"`
}

// PlaceStonePayload is received when placing a stone
type PlaceStonePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// RetractRespondPayload is received when responding to a retract request
type RetractRespondPayload struct {
	Agree bool `json:"agree"`
}

// SendChatPayload is received when sending a chat message
type SendChatPayload struct {
	Text string `json:"text"`
}

// SwitchRolePayload is received when switching roles
type SwitchRolePayload struct {
	Role string `json:"role"`
}

// ConfigureRoomPayload is received when changing room configurations
type ConfigureRoomPayload struct {
	Config RoomConfig `json:"config"`
}

// ServerMessage is the container for messages sent to clients
type ServerMessage struct {
	Type string      `json:"type"` // e.g. "room_list", "room_state", "chat_message", "error_message"
	Data interface{} `json:"data"`
}

// LobbyRoom represents a lightweight room metadata sent to the game lobby
type LobbyRoom struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"` // "waiting", "playing", "full"
	PlayerCount int    `json:"playerCount"`
	MaxPlayers  int    `json:"maxPlayers"`
	CreatorName string `json:"creatorName"`
}

// ChatMessage represents a chat message broadcast to room members
type ChatMessage struct {
	ID         string `json:"id"`
	SenderName string `json:"senderName"`
	Text       string `json:"text"`
	Timestamp  string `json:"timestamp"`
	IsSystem   bool   `json:"isSystem,omitempty"`
}
