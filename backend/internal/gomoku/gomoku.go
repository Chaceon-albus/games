package gomoku

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/olahol/melody"
)

const (
	defaultReconnectGrace = 60 * time.Second
	invalidSessionCode    = 4001
	replacedSessionCode   = 4002
)

type Manager struct {
	mu sync.RWMutex

	registry         *PlayerRegistry
	rooms            map[string]*Room
	memberships      map[string]string
	sessions         map[*melody.Session]*sessionBinding
	activeSessions   map[string]*melody.Session
	connectionSerial map[string]uint64
	melody           *melody.Melody
	reconnectGrace   time.Duration
}

type sessionBinding struct {
	uuid       string
	generation uint64
	limiter    *sessionRateLimiter
}

type sessionRateLimiter struct {
	mu sync.Mutex

	tokens         float64
	lastRefill     time.Time
	lastRoomAction time.Time
	lastChat       time.Time
}

type roomTarget struct {
	session *melody.Session
	uuid    string
}

var GameManager *Manager

func Init() *melody.Melody {
	ws := melody.New()
	ws.Config.MaxMessageSize = 4096
	manager := newManager(ws)
	GameManager = manager

	ws.HandleConnect(manager.handleConnect)
	ws.HandleDisconnect(manager.handleDisconnect)
	ws.HandleMessage(func(session *melody.Session, msg []byte) {
		manager.handleMessage(session, msg)
	})
	return ws
}

func newManager(ws *melody.Melody) *Manager {
	return &Manager{
		registry:         NewPlayerRegistry(),
		rooms:            make(map[string]*Room),
		memberships:      make(map[string]string),
		sessions:         make(map[*melody.Session]*sessionBinding),
		activeSessions:   make(map[string]*melody.Session),
		connectionSerial: make(map[string]uint64),
		melody:           ws,
		reconnectGrace:   defaultReconnectGrace,
	}
}

func (m *Manager) handleConnect(session *melody.Session) {
	uuid := session.Request.URL.Query().Get("uuid")
	player, ok := m.registry.Get(uuid)
	if !ok {
		slog.Warn("Gomoku: refused WebSocket connection with invalid session")
		m.sendError(session, ErrorInvalidSession, "Invalid session. Please register again.")
		_ = session.CloseWithMsg(websocket.FormatCloseMessage(invalidSessionCode, "invalid_session"))
		return
	}

	session.Set("uuid", uuid)

	m.mu.Lock()
	oldSession := m.activeSessions[uuid]
	generation := m.connectionSerial[uuid] + 1
	m.connectionSerial[uuid] = generation
	m.activeSessions[uuid] = session
	m.sessions[session] = &sessionBinding{
		uuid:       uuid,
		generation: generation,
		limiter:    newSessionRateLimiter(),
	}
	roomID := m.memberships[uuid]
	room := m.rooms[roomID]
	if roomID != "" && room == nil {
		delete(m.memberships, uuid)
		roomID = ""
	}
	if room != nil {
		room.MarkPlayerOnline(uuid)
	}
	m.mu.Unlock()

	session.Set("roomId", roomID)
	if oldSession != nil && oldSession != session {
		_ = oldSession.CloseWithMsg(websocket.FormatCloseMessage(replacedSessionCode, "session_replaced"))
	}

	slog.Info("Gomoku: player connected", slog.String("name", player.Name))
	if room != nil {
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】已重新连接。对局继续！", player.Name))
		return
	}
	m.sendRoomList(session)
}

func (m *Manager) handleDisconnect(session *melody.Session) {
	m.mu.Lock()
	binding, ok := m.sessions[session]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, session)
	if m.activeSessions[binding.uuid] != session {
		m.mu.Unlock()
		return
	}
	delete(m.activeSessions, binding.uuid)

	roomID := m.memberships[binding.uuid]
	room := m.rooms[roomID]
	if room == nil {
		m.mu.Unlock()
		return
	}
	result := room.DisconnectPlayer(binding.uuid)
	if result.Remove.Found {
		delete(m.memberships, binding.uuid)
		if result.Remove.Empty {
			delete(m.rooms, roomID)
		}
	}
	m.mu.Unlock()

	player, _ := m.registry.Get(binding.uuid)
	playerName := ""
	if player != nil {
		playerName = player.Name
	}
	if result.NeedsGrace {
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(roomID, fmt.Sprintf("【%s】连接断开！系统保留席位，等待重连（60秒内有效）...", playerName))
		time.AfterFunc(m.reconnectGrace, func() {
			m.handleDisconnectTimeout(binding.uuid, binding.generation, roomID)
		})
		return
	}
	if result.Remove.Found && !result.Remove.Empty {
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(roomID, fmt.Sprintf("【%s】离开了房间。", playerName))
		m.announceHostChange(roomID, result.Remove)
	}
	if result.Remove.Found {
		m.broadcastLobbyRooms()
	}
}

func (m *Manager) handleDisconnectTimeout(uuid string, generation uint64, roomID string) {
	m.mu.Lock()
	if m.connectionSerial[uuid] != generation || m.activeSessions[uuid] != nil || m.memberships[uuid] != roomID {
		m.mu.Unlock()
		return
	}
	room := m.rooms[roomID]
	if room == nil {
		m.mu.Unlock()
		return
	}
	result := room.RemoveOfflinePlayer(uuid)
	if !result.Found {
		m.mu.Unlock()
		return
	}
	delete(m.memberships, uuid)
	if result.Empty {
		delete(m.rooms, roomID)
	}
	m.mu.Unlock()

	player, _ := m.registry.Get(uuid)
	playerName := ""
	if player != nil {
		playerName = player.Name
	}
	if !result.Empty {
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(roomID, fmt.Sprintf("【%s】由于掉线超时被移出房间。", playerName))
		m.announceHostChange(roomID, result)
	}
	m.broadcastLobbyRooms()
}

func (m *Manager) handleMessage(session *melody.Session, raw []byte) {
	m.mu.RLock()
	binding := m.sessions[session]
	isCurrent := binding != nil && m.activeSessions[binding.uuid] == session
	m.mu.RUnlock()
	if !isCurrent {
		return
	}

	var message ClientMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		m.sendError(session, ErrorInvalidPayload, "Invalid message payload")
		return
	}
	if !binding.limiter.Allow(message.Action) {
		m.sendError(session, ErrorRateLimited, "Too many actions. Please slow down.")
		return
	}
	m.handleAction(session, binding.uuid, message)
}

func (m *Manager) handleAction(session *melody.Session, uuid string, message ClientMessage) {
	player, ok := m.registry.Get(uuid)
	if !ok {
		m.sendError(session, ErrorInvalidSession, "Invalid session")
		return
	}

	switch message.Action {
	case "create_room":
		var payload CreateRoomPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			m.sendError(session, ErrorInvalidPayload, "Invalid room configuration")
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" || getNicknameLength(name) > 40 || !ValidateRoomConfig(payload.Config) {
			m.sendError(session, ErrorInvalidRoomConfig, "Invalid room name or configuration")
			return
		}
		room, code := m.createRoom(uuid, name, payload.Config, player)
		if code != "" {
			m.sendRoomOperationError(session, code)
			return
		}
		session.Set("roomId", room.ID())
		m.broadcastRoomState(room)
		m.broadcastLobbyRooms()

	case "join_room":
		var payload JoinRoomPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			m.sendError(session, ErrorInvalidPayload, "Invalid join room payload")
			return
		}
		room, role, code := m.joinRoom(uuid, payload.RoomID, player)
		if code != "" {
			m.sendRoomOperationError(session, code)
			return
		}
		session.Set("roomId", room.ID())
		m.broadcastRoomState(room)
		if role != "existing" {
			m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】加入了房间。", player.Name))
			m.broadcastLobbyRooms()
		}

	case "leave_room":
		room, result, ok := m.leaveRoom(uuid)
		if !ok {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		session.Set("roomId", "")
		m.sendMessage(session, ServerMessage{Type: "room_state", Data: nil})
		m.sendRoomList(session)
		if !result.Empty {
			m.broadcastRoomState(room)
			m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】离开了房间。", player.Name))
			m.announceHostChange(room.ID(), result)
		}
		m.broadcastLobbyRooms()

	case "toggle_ready":
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.ToggleReady(uuid)
		if !result.Changed {
			m.sendError(session, ErrorPermissionDenied, "Ready state cannot be changed")
			return
		}
		m.broadcastRoomState(room)
		stateText := "取消准备"
		if result.IsReady {
			stateText = "准备"
		}
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】已%s", result.PlayerName, stateText))
		if result.Started {
			m.broadcastSystemMessage(room.ID(), "双方准备完毕，对局正式开始！")
			m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】执黑（先手），【%s】执白（后手）。", result.BlackName, result.WhiteName))
		}

	case "place_stone":
		var payload PlaceStonePayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			m.sendError(session, ErrorInvalidPayload, "Invalid stone placement")
			return
		}
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.PlaceStone(uuid, payload.Row, payload.Col)
		if !result.Placed {
			m.sendError(session, ErrorPermissionDenied, "Stone placement is not allowed")
			return
		}
		m.broadcastRoomState(room)
		if result.Finished && result.Winner == "draw" {
			m.broadcastSystemMessage(room.ID(), "棋盘已满！本局对决以平局结束。🤝")
		} else if result.Finished {
			m.broadcastSystemMessage(room.ID(), fmt.Sprintf("对局结束！【%s】达成五子相连，获得胜利！🎉", result.WinnerName))
		}

	case "request_retract":
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.RequestRetract(uuid)
		if !result.Requested {
			m.sendError(session, ErrorPermissionDenied, "Retract request is not allowed")
			return
		}
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】向对方提出了悔棋请求，等待对方同意...", result.RequesterName))

	case "retract_respond":
		var payload RetractRespondPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			m.sendError(session, ErrorInvalidPayload, "Invalid retract response")
			return
		}
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.HandleRetractResponse(uuid, payload.Agree)
		if !result.Valid {
			m.sendError(session, ErrorPermissionDenied, "Retract response is not allowed")
			return
		}
		m.broadcastRoomState(room)
		verb := "拒绝了"
		if result.Agreed {
			verb = "同意了"
		}
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】%s【%s】的悔棋请求。", result.ResponderName, verb, result.RequesterName))

	case "resign":
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.Resign(uuid)
		if !result.Resigned {
			m.sendError(session, ErrorPermissionDenied, "Resignation is not allowed")
			return
		}
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】认输。", result.PlayerName))

	case "configure_room":
		var payload ConfigureRoomPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil || !ValidateRoomConfig(payload.Config) {
			m.sendError(session, ErrorInvalidRoomConfig, "Invalid room configuration")
			return
		}
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		result := room.UpdateConfig(uuid, payload.Config)
		if !result.Updated {
			m.sendError(session, ErrorPermissionDenied, "Only the host can update room settings")
			return
		}
		m.broadcastRoomState(room)
		m.broadcastLobbyRooms()
		if result.PromotedName != "" {
			m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】已自动补位成为对局玩家。", result.PromotedName))
		}

	case "claim_seat":
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		if !room.ClaimSeat(uuid) {
			m.sendError(session, ErrorPermissionDenied, "The opponent seat is not available")
			return
		}
		m.broadcastRoomState(room)
		m.broadcastSystemMessage(room.ID(), fmt.Sprintf("【%s】加入了对局席位。", player.Name))
		m.broadcastLobbyRooms()

	case "send_chat":
		var payload SendChatPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			m.sendError(session, ErrorInvalidPayload, "Invalid chat payload")
			return
		}
		text := strings.TrimSpace(payload.Text)
		if text == "" || utf8.RuneCountInString(text) > MaxChatLength {
			m.sendError(session, ErrorInvalidPayload, "Chat messages must contain 1 to 100 characters")
			return
		}
		room := m.currentRoom(uuid)
		if room == nil {
			m.sendError(session, ErrorNotRoomMember, "You are not in a room")
			return
		}
		sender, code := room.ChatSender(uuid)
		if code != "" {
			message := "Chat is not allowed"
			if code == ErrorChatDisabled {
				message = "房间已禁用聊天功能"
			} else if code == ErrorSpectatorChat {
				message = "观战者不能发送聊天消息"
			}
			m.sendError(session, code, message)
			return
		}
		chatMessage := ChatMessage{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			SenderID:   sender.PublicID,
			SenderName: sender.Name,
			Text:       text,
			Timestamp:  time.Now().Format("15:04"),
		}
		m.broadcastChatMessage(room.ID(), chatMessage)

	case "list_rooms":
		m.sendRoomList(session)

	default:
		m.sendError(session, ErrorInvalidPayload, "Unknown action")
	}
}

func (m *Manager) createRoom(uuid, name string, config RoomConfig, player *Player) (*Room, string) {
	for attempts := 0; attempts < 16; attempts++ {
		roomID, err := generateRoomID()
		if err != nil {
			return nil, ErrorInvalidPayload
		}
		m.mu.Lock()
		if _, exists := m.memberships[uuid]; exists {
			m.mu.Unlock()
			return nil, ErrorAlreadyInRoom
		}
		if len(m.rooms) >= MaxRooms {
			m.mu.Unlock()
			return nil, ErrorRoomLimit
		}
		if _, collision := m.rooms[roomID]; collision {
			m.mu.Unlock()
			continue
		}
		room := NewRoom(roomID, name, player)
		room.SetInitialConfig(config)
		m.rooms[roomID] = room
		m.memberships[uuid] = roomID
		m.mu.Unlock()
		return room, ""
	}
	return nil, ErrorRoomLimit
}

func (m *Manager) joinRoom(uuid, roomID string, player *Player) (*Room, string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if currentRoomID, exists := m.memberships[uuid]; exists {
		if currentRoomID == roomID {
			room := m.rooms[roomID]
			if room != nil {
				return room, "existing", ""
			}
		}
		return nil, "", ErrorAlreadyInRoom
	}
	room := m.rooms[roomID]
	if room == nil {
		return nil, "", ErrorRoomNotFound
	}
	role, err := room.AddPlayer(player)
	if errorsIsSpectatorCapacity(err) {
		return nil, "", ErrorRoomCapacity
	}
	if err != nil {
		return nil, "", ErrorInvalidPayload
	}
	m.memberships[uuid] = roomID
	return room, role, ""
}

func errorsIsSpectatorCapacity(err error) bool {
	return err == ErrSpectatorCapacity
}

func (m *Manager) leaveRoom(uuid string) (*Room, RemoveResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	roomID, ok := m.memberships[uuid]
	if !ok {
		return nil, RemoveResult{}, false
	}
	room := m.rooms[roomID]
	if room == nil {
		delete(m.memberships, uuid)
		return nil, RemoveResult{}, false
	}
	result := room.RemovePlayer(uuid)
	delete(m.memberships, uuid)
	if result.Empty {
		delete(m.rooms, roomID)
	}
	return room, result, result.Found
}

func (m *Manager) currentRoom(uuid string) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rooms[m.memberships[uuid]]
}

func (m *Manager) sendRoomOperationError(session *melody.Session, code string) {
	messages := map[string]string{
		ErrorAlreadyInRoom: "请先离开当前房间",
		ErrorRoomNotFound:  "房间不存在，可能已被销毁",
		ErrorRoomCapacity:  "房间观战人数已达到上限",
		ErrorRoomLimit:     "服务器房间数量已达到上限",
	}
	message := messages[code]
	if message == "" {
		message = "Room operation failed"
	}
	m.sendError(session, code, message)
}

func (m *Manager) roomTargets(roomID string) []roomTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	targets := make([]roomTarget, 0)
	for uuid, session := range m.activeSessions {
		if m.memberships[uuid] == roomID {
			targets = append(targets, roomTarget{session: session, uuid: uuid})
		}
	}
	return targets
}

func (m *Manager) sendRoomState(session *melody.Session, room *Room, uuid string) {
	m.sendMessage(session, ServerMessage{Type: "room_state", Data: room.Snapshot(uuid)})
}

func (m *Manager) broadcastRoomState(room *Room) {
	for _, target := range m.roomTargets(room.ID()) {
		m.sendRoomState(target.session, room, target.uuid)
	}
}

func (m *Manager) sendRoomList(session *melody.Session) {
	m.sendMessage(session, ServerMessage{Type: "room_list", Data: m.getLobbyRooms()})
}

func (m *Manager) broadcastLobbyRooms() {
	rooms := m.getLobbyRooms()
	message, err := json.Marshal(ServerMessage{Type: "room_list", Data: rooms})
	if err != nil {
		return
	}
	m.mu.RLock()
	targets := make([]*melody.Session, 0)
	for uuid, session := range m.activeSessions {
		if _, inRoom := m.memberships[uuid]; !inRoom {
			targets = append(targets, session)
		}
	}
	m.mu.RUnlock()
	for _, session := range targets {
		_ = session.Write(message)
	}
}

func (m *Manager) getLobbyRooms() []LobbyRoom {
	m.mu.RLock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.mu.RUnlock()

	list := make([]LobbyRoom, 0, len(rooms))
	for _, room := range rooms {
		list = append(list, room.LobbySnapshot())
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

func (m *Manager) broadcastSystemMessage(roomID, text string) {
	m.broadcastChatMessage(roomID, ChatMessage{
		ID:         fmt.Sprintf("sys_%d", time.Now().UnixNano()),
		SenderName: "系统",
		Text:       text,
		Timestamp:  time.Now().Format("15:04"),
		IsSystem:   true,
	})
}

func (m *Manager) broadcastChatMessage(roomID string, chatMessage ChatMessage) {
	message, err := json.Marshal(ServerMessage{Type: "chat_message", Data: chatMessage})
	if err != nil {
		return
	}
	for _, target := range m.roomTargets(roomID) {
		_ = target.session.Write(message)
	}
}

func (m *Manager) announceHostChange(roomID string, result RemoveResult) {
	if result.HostChanged && result.NewHostName != "" {
		m.broadcastSystemMessage(roomID, fmt.Sprintf("提示：房主已离开，【%s】成为了新房主！👑", result.NewHostName))
	}
}

func (m *Manager) sendError(session *melody.Session, code, message string) {
	m.sendMessage(session, ServerMessage{
		Type: "error_message",
		Data: ErrorPayload{Code: code, Message: message},
	})
}

func (m *Manager) sendMessage(session *melody.Session, message ServerMessage) {
	payload, err := json.Marshal(message)
	if err != nil {
		return
	}
	_ = session.Write(payload)
}

func RegisterHandlers(api *gin.RouterGroup) {
	api.POST("/register", func(context *gin.Context) {
		var request struct {
			Nickname string `json:"nickname"`
		}
		if err := context.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Nickname) == "" {
			context.JSON(http.StatusBadRequest, gin.H{"error": "Nickname is required"})
			return
		}
		nickname := truncateDisplayName(strings.TrimSpace(request.Nickname), 20)
		player, err := GameManager.registry.Register(nickname)
		if err != nil {
			context.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
			return
		}
		context.JSON(http.StatusOK, gin.H{
			"uuid":     player.UUID,
			"playerId": player.PublicID,
			"nickname": player.Name,
		})
	})

	api.POST("/verify", func(context *gin.Context) {
		var request struct {
			UUID string `json:"uuid"`
		}
		if err := context.ShouldBindJSON(&request); err != nil {
			context.JSON(http.StatusBadRequest, gin.H{"error": "UUID is required"})
			return
		}
		player, ok := GameManager.registry.Get(request.UUID)
		if !ok {
			context.JSON(http.StatusOK, gin.H{"valid": false})
			return
		}
		context.JSON(http.StatusOK, gin.H{
			"valid":    true,
			"playerId": player.PublicID,
			"nickname": player.Name,
		})
	})
}

func newSessionRateLimiter() *sessionRateLimiter {
	return &sessionRateLimiter{tokens: 20, lastRefill: time.Now()}
}

func (limiter *sessionRateLimiter) Allow(action string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(limiter.lastRefill).Seconds()
	limiter.tokens += elapsed * 10
	if limiter.tokens > 20 {
		limiter.tokens = 20
	}
	limiter.lastRefill = now
	if limiter.tokens < 1 {
		return false
	}
	if (action == "create_room" || action == "join_room" || action == "claim_seat") && now.Sub(limiter.lastRoomAction) < time.Second {
		return false
	}
	if action == "send_chat" && now.Sub(limiter.lastChat) < time.Second {
		return false
	}
	limiter.tokens--
	if action == "create_room" || action == "join_room" || action == "claim_seat" {
		limiter.lastRoomAction = now
	}
	if action == "send_chat" {
		limiter.lastChat = now
	}
	return true
}

func generateRoomID() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func truncateDisplayName(value string, maxLength int) string {
	result := strings.Builder{}
	length := 0
	for _, current := range value {
		unit := 1
		if current > 127 {
			unit = 2
		}
		if length+unit > maxLength {
			break
		}
		result.WriteRune(current)
		length += unit
	}
	return result.String()
}

func getNicknameLength(value string) int {
	length := 0
	for _, current := range value {
		if current > 127 {
			length += 2
		} else {
			length++
		}
	}
	return length
}
