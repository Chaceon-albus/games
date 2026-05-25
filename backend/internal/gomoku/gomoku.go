package gomoku

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
)

// Manager coordinates the overall multiplayer Gomoku server state
type Manager struct {
	mu       sync.RWMutex
	registry *PlayerRegistry
	rooms    map[string]*Room
	sessions map[*melody.Session]string
	melody   *melody.Melody
}

// Global instance of the Gomoku GameManager
var GameManager *Manager

// Init initializes the Gomoku manager and returns the melody WebSocket handler
func Init() *melody.Melody {
	m := melody.New()
	m.Config.MaxMessageSize = 4096

	GameManager = &Manager{
		registry: NewPlayerRegistry(),
		rooms:    make(map[string]*Room),
		sessions: make(map[*melody.Session]string),
		melody:   m,
	}

	// Connection upgraded hook
	m.HandleConnect(func(s *melody.Session) {
		uuid := s.Request.URL.Query().Get("uuid")
		slog.Debug("Gomoku: WS connecting", slog.String("uuid", uuid))

		player, ok := GameManager.registry.Get(uuid)
		if !ok {
			slog.Warn("Gomoku: Refused WS connection, invalid UUID", slog.String("uuid", uuid))
			_ = s.Write([]byte(`{"type":"error_message","data":{"message":"Invalid session. Please re-register."}}`))
			s.Close()
			return
		}

		GameManager.mu.Lock()
		GameManager.sessions[s] = uuid
		player.Session = s
		player.IsOffline = false
		GameManager.mu.Unlock()

		s.Set("uuid", uuid)
		s.Set("roomId", "") // Initially in lobby

		slog.Info("Gomoku: Player connected via WS", slog.String("name", player.Name), slog.String("uuid", uuid))

		// Check if player is already in an active room (reconnection recovery)
		var activeRoom *Room
		GameManager.mu.RLock()
		for _, r := range GameManager.rooms {
			if (r.Host != nil && r.Host.UUID == uuid) || (r.Opponent != nil && r.Opponent.UUID == uuid) {
				activeRoom = r
				break
			}
			for _, spec := range r.Spectators {
				if spec.UUID == uuid {
					activeRoom = r
					break
				}
			}
		}
		GameManager.mu.RUnlock()

		if activeRoom != nil {
			// Restore session's room mapping
			s.Set("roomId", activeRoom.ID)
			activeRoom.SetPlayerOffline(uuid, false)

			// Broadcast recovered state
			GameManager.broadcastRoomState(activeRoom)

			// Notify room of reconnection
			GameManager.broadcastSystemMessage(activeRoom.ID, fmt.Sprintf("【%s】已重新连接。对局继续！", player.Name))
		} else {
			// Send initial lobby list
			GameManager.sendRoomList(s)
		}
	})

	// Disconnection hook
	m.HandleDisconnect(func(s *melody.Session) {
		val, _ := s.Get("uuid")
		uuid, _ := val.(string)
		if uuid == "" {
			return
		}

		player, ok := GameManager.registry.Get(uuid)
		if !ok {
			return
		}

		GameManager.mu.Lock()
		delete(GameManager.sessions, s)
		player.Session = nil
		GameManager.mu.Unlock()

		slog.Info("Gomoku: Player disconnected from WS", slog.String("name", player.Name), slog.String("uuid", uuid))

		// Check if they were in a room
		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		GameManager.mu.RLock()
		room, hasRoom := GameManager.rooms[roomID]
		GameManager.mu.RUnlock()

		if !hasRoom {
			return
		}

		player.DisconnectAt = time.Now()
		room.SetPlayerOffline(uuid, true)

		// If the game is waiting, remove immediately to keep room active
		if room.Status == "waiting" {
			empty := room.RemovePlayer(uuid)
			if empty {
				GameManager.mu.Lock()
				delete(GameManager.rooms, roomID)
				GameManager.mu.Unlock()
			} else {
				GameManager.broadcastRoomState(room)
				GameManager.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】离开了房间。", player.Name))
			}
			GameManager.broadcastLobbyRooms()
		} else {
			// Game is playing: start 60-second recovery timer
			GameManager.broadcastRoomState(room)
			GameManager.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】连接断开！系统保留席位，等待重连（60秒内有效）...", player.Name))

			go func(p *Player, r *Room, disconnectTime time.Time) {
				time.Sleep(60 * time.Second)
				GameManager.handleDisconnectTimeout(p, r, disconnectTime)
			}(player, room, player.DisconnectAt)
		}
	})

	// Message dispatcher hook
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		val, _ := s.Get("uuid")
		uuid, _ := val.(string)
		if uuid == "" {
			return
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			slog.Error("Gomoku: Failed to unmarshal client message", slog.String("error", err.Error()))
			return
		}

		GameManager.handleAction(s, uuid, clientMsg)
	})

	return m
}

// RegisterHandlers maps REST endpoints for registration & verification in Gin
func RegisterHandlers(api *gin.RouterGroup) {
	api.POST("/register", func(c *gin.Context) {
		var req struct {
			Nickname string `json:"nickname"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Nickname) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Nickname is required"})
			return
		}

		nickname := strings.TrimSpace(req.Nickname)
		if getNicknameLength(nickname) > 20 {
			truncated := ""
			length := 0
			for _, r := range nickname {
				unit := 1
				if r > 127 {
					unit = 2
				}
				if length+unit > 20 {
					break
				}
				truncated += string(r)
				length += unit
			}
			nickname = truncated
		}

		player, err := GameManager.registry.Register(nickname)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
			return
		}

		slog.Info("Gomoku: Registered new player", slog.String("name", player.Name), slog.String("uuid", player.UUID))
		c.JSON(http.StatusOK, gin.H{
			"uuid":     player.UUID,
			"nickname": player.Name,
		})
	})

	api.POST("/verify", func(c *gin.Context) {
		var req struct {
			UUID string `json:"uuid"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "UUID is required"})
			return
		}

		player, ok := GameManager.registry.Get(req.UUID)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"valid": false})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"valid":    true,
			"nickname": player.Name,
		})
	})
}

// handleDisconnectTimeout executes if a player fails to reconnect within 60s
func (m *Manager) handleDisconnectTimeout(p *Player, r *Room, disconnectTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check if player actually reconnected, or if this check belongs to an older disconnect session
	if !p.IsOffline || !p.DisconnectAt.Equal(disconnectTime) {
		return
	}

	// Verify room still exists in manager
	if _, ok := m.rooms[r.ID]; !ok {
		return
	}

	// Verify the game is still playing and the player is actually in the room
	if r.Status != "playing" || !r.HasPlayer(p.UUID) {
		return
	}

	slog.Info("Gomoku: Player reconnect timeout, removing from room", slog.String("name", p.Name), slog.String("roomId", r.ID))

	oldHostUUID := ""
	if r.Host != nil {
		oldHostUUID = r.Host.UUID
	}

	empty := r.RemovePlayer(p.UUID)
	if empty {
		delete(m.rooms, r.ID)
	} else {
		m.broadcastRoomState(r)
		m.broadcastSystemMessage(r.ID, fmt.Sprintf("【%s】由于掉线超时被移出房间。", p.Name))
		if r.Host != nil && r.Host.UUID != oldHostUUID {
			m.broadcastSystemMessage(r.ID, fmt.Sprintf("提示：房主已离开，【%s】成为了新房主！👑", r.Host.Name))
		}
	}
	m.broadcastLobbyRooms()
}

// handleAction dispatches WS actions
func (m *Manager) handleAction(s *melody.Session, uuid string, msg ClientMessage) {
	player, ok := m.registry.Get(uuid)
	if !ok {
		return
	}

	switch msg.Action {
	case "create_room":
		var payload CreateRoomPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			m.sendError(s, "Invalid room configuration payload")
			return
		}

		roomID := fmt.Sprintf("%06d", rand.Intn(1000000))
		room := NewRoom(roomID, payload.Name, player)
		room.Config = payload.Config

		m.mu.Lock()
		m.rooms[roomID] = room
		m.mu.Unlock()

		s.Set("roomId", roomID)
		m.broadcastRoomState(room)
		m.broadcastLobbyRooms()

	case "join_room":
		var payload JoinRoomPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			m.sendError(s, "Invalid join room payload")
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[payload.RoomID]
		m.mu.RUnlock()

		if !ok {
			m.sendError(s, "房间不存在，可能已被销毁")
			return
		}

		room.AddPlayer(player)
		s.Set("roomId", room.ID)
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】加入了房间。", player.Name))
		m.broadcastLobbyRooms()

	case "leave_room":
		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()

		if !ok {
			return
		}

		oldHostUUID := ""
		if room.Host != nil {
			oldHostUUID = room.Host.UUID
		}

		empty := room.RemovePlayer(uuid)
		s.Set("roomId", "")

		// Send nil room state back to the leaving player so their UI switches back to the Lobby
		leaveMsg := ServerMessage{
			Type: "room_state",
			Data: nil,
		}
		leaveMsgBytes, _ := json.Marshal(leaveMsg)
		_ = s.Write(leaveMsgBytes)

		m.sendRoomList(s)

		if empty {
			m.mu.Lock()
			delete(m.rooms, roomID)
			m.mu.Unlock()
		} else {
			m.broadcastRoomState(room)
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】离开了房间。", player.Name))
			if room.Host != nil && room.Host.UUID != oldHostUUID {
				m.broadcastSystemMessage(room.ID, fmt.Sprintf("提示：房主已离开，【%s】成为了新房主！👑", room.Host.Name))
			}
		}
		m.broadcastLobbyRooms()

	case "toggle_ready":
		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		oldStatus := room.Status
		room.ToggleReady(uuid)
		m.broadcastRoomState(room)

		// System broadcast ready status
		var p *Player
		if room.Host != nil && room.Host.UUID == uuid {
			p = room.Host
		} else if room.Opponent != nil && room.Opponent.UUID == uuid {
			p = room.Opponent
		}
		if p != nil {
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】已%s", p.Name, map[bool]string{true: "准备", false: "取消准备"}[p.IsReady]))
		}

		// Notify game started
		if oldStatus == "waiting" && room.Status == "playing" {
			var blackName, whiteName string
			if room.HostColor == "black" {
				blackName = room.Host.Name
				whiteName = room.Opponent.Name
			} else {
				blackName = room.Opponent.Name
				whiteName = room.Host.Name
			}
			m.broadcastSystemMessage(room.ID, "双方准备完毕，对局正式开始！")
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】执黑（先手），【%s】执白（后手）。", blackName, whiteName))
		}

	case "place_stone":
		var payload PlaceStonePayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			return
		}

		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		oldLen := len(room.History)
		win := room.PlaceStone(uuid, payload.Row, payload.Col)
		if len(room.History) > oldLen {
			m.broadcastRoomState(room)

			if win {
				if room.Winner == "draw" {
					m.broadcastSystemMessage(room.ID, "棋盘已满！本局对决以平局结束。🤝")
				} else {
					var winnerName string
					if room.Winner == "black" {
						if room.HostColor == "black" {
							winnerName = room.Host.Name
						} else {
							winnerName = room.Opponent.Name
						}
					} else {
						if room.HostColor == "white" {
							winnerName = room.Host.Name
						} else {
							winnerName = room.Opponent.Name
						}
					}
					m.broadcastSystemMessage(room.ID, fmt.Sprintf("对局结束！【%s】达成五子相连，获得胜利！🎉", winnerName))
				}
			}
		}

	case "request_retract":
		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		if room.RequestRetract(uuid) {
			m.broadcastRoomState(room)
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】向对方提出了悔棋请求，等待对方同意...", player.Name))
		}

	case "retract_respond":
		var payload RetractRespondPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			return
		}

		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		requesterUUID := room.RetractRequester
		if requesterUUID == "" {
			return
		}

		requester, hasReq := m.registry.Get(requesterUUID)
		if !hasReq {
			return
		}

		success := room.HandleRetractResponse(uuid, payload.Agree)
		m.broadcastRoomState(room)

		if payload.Agree && success {
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】同意了【%s】的悔棋请求。", player.Name, requester.Name))
		} else {
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】拒绝了【%s】的悔棋请求。", player.Name, requester.Name))
		}

	case "resign":
		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		if room.Resign(uuid) {
			m.broadcastRoomState(room)
			m.broadcastSystemMessage(room.ID, fmt.Sprintf("【%s】认输。", player.Name))
		}

	case "configure_room":
		var payload ConfigureRoomPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			return
		}

		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		if room.UpdateConfig(uuid, payload.Config) {
			m.broadcastRoomState(room)
			m.broadcastLobbyRooms()
		}

	case "send_chat":
		var payload SendChatPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			return
		}

		roomVal, _ := s.Get("roomId")
		roomID, _ := roomVal.(string)
		if roomID == "" {
			return
		}

		m.mu.RLock()
		room, ok := m.rooms[roomID]
		m.mu.RUnlock()
		if !ok {
			return
		}

		// Respect disable chat setting
		if room.Config.DisableChat {
			m.sendError(s, "房间已禁用聊天功能")
			return
		}

		chatMsg := ChatMessage{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			SenderName: player.Name,
			Text:       payload.Text,
			Timestamp:  time.Now().Format("15:04"),
		}

		serverMsg := ServerMessage{
			Type: "chat_message",
			Data: chatMsg,
		}

		msgBytes, _ := json.Marshal(serverMsg)

		// Broadcast to all room members EXCEPT the sender
		_ = m.melody.BroadcastFilter(msgBytes, func(dest *melody.Session) bool {
			if dest == s {
				return false
			}
			val, _ := dest.Get("roomId")
			destRoomID, _ := val.(string)
			return destRoomID == room.ID
		})

	case "list_rooms":
		m.sendRoomList(s)
	}
}

// sendRoomList sends the lobby list to a specific session
func (m *Manager) sendRoomList(s *melody.Session) {
	lobbyRooms := m.getLobbyRooms()
	msg := ServerMessage{
		Type: "room_list",
		Data: lobbyRooms,
	}
	msgBytes, _ := json.Marshal(msg)
	_ = s.Write(msgBytes)
}

// broadcastLobbyRooms broadcasts the lobby list to all active sessions in the lobby
func (m *Manager) broadcastLobbyRooms() {
	lobbyRooms := m.getLobbyRooms()
	msg := ServerMessage{
		Type: "room_list",
		Data: lobbyRooms,
	}
	msgBytes, _ := json.Marshal(msg)

	_ = m.melody.BroadcastFilter(msgBytes, func(s *melody.Session) bool {
		val, _ := s.Get("roomId")
		roomId, _ := val.(string)
		return roomId == ""
	})
}

// getLobbyRooms aggregates lobby metadata
func (m *Manager) getLobbyRooms() []LobbyRoom {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]LobbyRoom, 0, len(m.rooms))
	for _, r := range m.rooms {
		playerCount := 0
		if r.Host != nil {
			playerCount++
		}
		if r.Opponent != nil {
			playerCount++
		}
		playerCount += len(r.Spectators)

		creatorName := ""
		if r.Host != nil {
			creatorName = r.Host.Name
		}

		list = append(list, LobbyRoom{
			ID:          r.ID,
			Name:        r.Name,
			Status:      r.Status,
			PlayerCount: playerCount,
			MaxPlayers:  2,
			CreatorName: creatorName,
		})
	}
	return list
}

// broadcastRoomState sends the updated Room struct to everyone inside the room
func (m *Manager) broadcastRoomState(room *Room) {
	msg := ServerMessage{
		Type: "room_state",
		Data: room,
	}
	msgBytes, _ := json.Marshal(msg)

	_ = m.melody.BroadcastFilter(msgBytes, func(s *melody.Session) bool {
		val, _ := s.Get("roomId")
		roomId, _ := val.(string)
		return roomId == room.ID
	})
}

// broadcastSystemMessage sends a system banner text to all users in the room
func (m *Manager) broadcastSystemMessage(roomID string, text string) {
	chatMsg := ChatMessage{
		ID:         fmt.Sprintf("sys_%d", time.Now().UnixNano()),
		SenderName: "系统",
		Text:       text,
		Timestamp:  time.Now().Format("15:04"),
		IsSystem:   true,
	}

	serverMsg := ServerMessage{
		Type: "chat_message",
		Data: chatMsg,
	}

	msgBytes, _ := json.Marshal(serverMsg)

	_ = m.melody.BroadcastFilter(msgBytes, func(s *melody.Session) bool {
		val, _ := s.Get("roomId")
		destRoomID, _ := val.(string)
		return destRoomID == roomID
	})
}

// sendError unicasts an error warning to a single session
func (m *Manager) sendError(s *melody.Session, errMsg string) {
	msg := ServerMessage{
		Type: "error_message",
		Data: gin.H{"message": errMsg},
	}
	msgBytes, _ := json.Marshal(msg)
	_ = s.Write(msgBytes)
}

// getNicknameLength calculates nickname length where non-ASCII counts as 2 units, ASCII counts as 1 unit
func getNicknameLength(s string) int {
	length := 0
	for _, r := range s {
		if r > 127 {
			length += 2
		} else {
			length += 1
		}
	}
	return length
}
