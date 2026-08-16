package gomoku

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

var ErrSpectatorCapacity = errors.New("spectator capacity reached")

type RoomConfig struct {
	AutoJoinSpectator bool   `json:"autoJoinSpectator"`
	DisableChat       bool   `json:"disableChat"`
	ColorMode         string `json:"colorMode"`
}

type Move struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Player string `json:"player"`
}

type Coord struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type roomMember struct {
	player    *Player
	isReady   bool
	isOffline bool
}

type Room struct {
	mu sync.RWMutex

	id                      string
	name                    string
	status                  string
	host                    *roomMember
	opponent                *roomMember
	spectators              []*roomMember
	config                  RoomConfig
	history                 []Move
	turn                    string
	winner                  string
	winningLine             []Coord
	hostColor               string
	opponentColor           string
	consecutiveGames        int
	retractRequester        string
	retractRequesterName    string
	lastRetractTimeByPlayer map[string]time.Time
}

type RemoveResult struct {
	Found       bool
	Empty       bool
	HostChanged bool
	NewHostName string
}

type DisconnectResult struct {
	NeedsGrace bool
	Remove     RemoveResult
}

type ReadyResult struct {
	Changed    bool
	PlayerName string
	IsReady    bool
	Started    bool
	BlackName  string
	WhiteName  string
}

type PlaceResult struct {
	Placed     bool
	Finished   bool
	Winner     string
	WinnerName string
}

type RetractRequestResult struct {
	Requested     bool
	RequesterName string
}

type RetractResponseResult struct {
	Valid         bool
	Agreed        bool
	ResponderName string
	RequesterName string
}

type ResignResult struct {
	Resigned   bool
	PlayerName string
}

type ConfigUpdateResult struct {
	Updated      bool
	PromotedName string
}

func ValidateRoomConfig(config RoomConfig) bool {
	return config.ColorMode == "alternating" || config.ColorMode == "random"
}

func NewRoom(id, name string, creator *Player) *Room {
	return &Room{
		id:                      id,
		name:                    name,
		status:                  "waiting",
		host:                    newRoomMember(creator),
		config:                  RoomConfig{ColorMode: "alternating"},
		history:                 make([]Move, 0),
		spectators:              make([]*roomMember, 0),
		turn:                    "black",
		hostColor:               "black",
		opponentColor:           "white",
		lastRetractTimeByPlayer: make(map[string]time.Time),
	}
}

func newRoomMember(player *Player) *roomMember {
	return &roomMember{player: player}
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) SetInitialConfig(config RoomConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

func (r *Room) Snapshot(viewerUUID string) RoomStateView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	history := make([]Move, len(r.history))
	copy(history, r.history)
	winningLine := make([]Coord, len(r.winningLine))
	copy(winningLine, r.winningLine)

	view := RoomStateView{
		ID:                     r.id,
		Name:                   r.name,
		Status:                 r.status,
		Host:                   memberView(r.host),
		Opponent:               memberView(r.opponent),
		Spectators:             make([]PlayerView, 0, len(r.spectators)),
		Config:                 r.config,
		History:                history,
		Turn:                   r.turn,
		Winner:                 r.winner,
		WinningLine:            winningLine,
		HostColor:              r.hostColor,
		OpponentColor:          r.opponentColor,
		ConsecutiveGames:       r.consecutiveGames,
		RetractPending:         r.retractRequester != "",
		RetractRequesterName:   r.retractRequesterName,
		RetractRequestedBySelf: r.retractRequester == viewerUUID,
	}

	for _, spectator := range r.spectators {
		view.Spectators = append(view.Spectators, *memberView(spectator))
	}

	if r.host != nil && r.host.player.UUID == viewerUUID {
		view.Self = SelfView{PlayerID: r.host.player.PublicID, Role: "host", Color: r.hostColor}
	} else if r.opponent != nil && r.opponent.player.UUID == viewerUUID {
		view.Self = SelfView{PlayerID: r.opponent.player.PublicID, Role: "opponent", Color: r.opponentColor}
	} else {
		for _, spectator := range r.spectators {
			if spectator.player.UUID == viewerUUID {
				view.Self = SelfView{PlayerID: spectator.player.PublicID, Role: "spectator"}
				break
			}
		}
	}

	return view
}

func memberView(member *roomMember) *PlayerView {
	if member == nil {
		return nil
	}
	return &PlayerView{
		ID:        member.player.PublicID,
		Name:      member.player.Name,
		Avatar:    member.player.Avatar,
		IsReady:   member.isReady,
		IsOffline: member.isOffline,
	}
}

func (r *Room) LobbySnapshot() LobbyRoom {
	r.mu.RLock()
	defer r.mu.RUnlock()

	playerCount := 0
	creatorName := ""
	if r.host != nil {
		playerCount++
		creatorName = r.host.player.Name
	}
	if r.opponent != nil {
		playerCount++
	}
	return LobbyRoom{
		ID:             r.id,
		Name:           r.name,
		Status:         r.status,
		PlayerCount:    playerCount,
		MaxPlayers:     2,
		SpectatorCount: len(r.spectators),
		MaxSpectators:  MaxSpectatorsPerRoom,
		CreatorName:    creatorName,
	}
}

func (r *Room) HasPlayer(uuid string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, _, ok := r.findMemberLocked(uuid)
	return ok
}

func (r *Room) AddPlayer(player *Player) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, role, ok := r.findMemberLocked(player.UUID); ok {
		return role, nil
	}

	member := newRoomMember(player)
	if r.host == nil {
		r.host = member
		return "host", nil
	}
	if r.opponent == nil {
		r.opponent = member
		return "opponent", nil
	}
	if len(r.spectators) >= MaxSpectatorsPerRoom {
		return "", ErrSpectatorCapacity
	}
	r.spectators = append(r.spectators, member)
	return "spectator", nil
}

func (r *Room) RemovePlayer(uuid string) RemoveResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.removePlayerLocked(uuid)
}

func (r *Room) removePlayerLocked(uuid string) RemoveResult {
	oldHostUUID := ""
	if r.host != nil {
		oldHostUUID = r.host.player.UUID
	}

	wasActive := false
	found := false
	if r.host != nil && r.host.player.UUID == uuid {
		r.host = nil
		wasActive = true
		found = true
	} else if r.opponent != nil && r.opponent.player.UUID == uuid {
		r.opponent = nil
		wasActive = true
		found = true
	} else {
		for i, spectator := range r.spectators {
			if spectator.player.UUID == uuid {
				r.spectators = append(r.spectators[:i], r.spectators[i+1:]...)
				found = true
				break
			}
		}
	}

	if !found {
		return RemoveResult{Empty: r.isEmptyLocked()}
	}

	if wasActive && r.status == "playing" {
		r.resetMatchLocked()
	}
	delete(r.lastRetractTimeByPlayer, uuid)
	r.handleVacancyLocked()

	newHostUUID := ""
	newHostName := ""
	if r.host != nil {
		newHostUUID = r.host.player.UUID
		newHostName = r.host.player.Name
	}
	return RemoveResult{
		Found:       true,
		Empty:       r.isEmptyLocked(),
		HostChanged: oldHostUUID != "" && oldHostUUID != newHostUUID,
		NewHostName: newHostName,
	}
}

func (r *Room) DisconnectPlayer(uuid string) DisconnectResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	member, role, ok := r.findMemberLocked(uuid)
	if !ok {
		return DisconnectResult{}
	}
	if r.status == "playing" && role != "spectator" {
		member.isOffline = true
		return DisconnectResult{NeedsGrace: true}
	}
	return DisconnectResult{Remove: r.removePlayerLocked(uuid)}
}

func (r *Room) MarkPlayerOnline(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	member, _, ok := r.findMemberLocked(uuid)
	if !ok {
		return false
	}
	member.isOffline = false
	return true
}

func (r *Room) RemoveOfflinePlayer(uuid string) RemoveResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	member, _, ok := r.findMemberLocked(uuid)
	if !ok || !member.isOffline {
		return RemoveResult{Empty: r.isEmptyLocked()}
	}
	return r.removePlayerLocked(uuid)
}

func (r *Room) ToggleReady(uuid string) ReadyResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == "playing" {
		return ReadyResult{}
	}
	member, role, ok := r.findMemberLocked(uuid)
	if !ok || role == "spectator" || member.isOffline {
		return ReadyResult{}
	}

	member.isReady = !member.isReady
	result := ReadyResult{Changed: true, PlayerName: member.player.Name, IsReady: member.isReady}
	if r.host == nil || r.opponent == nil || !r.host.isReady || !r.opponent.isReady {
		return result
	}

	r.status = "playing"
	r.history = make([]Move, 0)
	r.winner = ""
	r.winningLine = nil
	r.turn = "black"
	r.retractRequester = ""
	r.retractRequesterName = ""
	if r.config.ColorMode == "random" {
		if rand.Intn(2) == 0 {
			r.hostColor, r.opponentColor = "black", "white"
		} else {
			r.hostColor, r.opponentColor = "white", "black"
		}
	} else if r.consecutiveGames == 0 {
		r.hostColor, r.opponentColor = "black", "white"
	} else {
		r.hostColor, r.opponentColor = r.opponentColor, r.hostColor
	}
	r.consecutiveGames++
	result.Started = true
	result.BlackName = r.playerNameByColorLocked("black")
	result.WhiteName = r.playerNameByColorLocked("white")
	return result
}

func (r *Room) PlaceStone(uuid string, row, col int) PlaceResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != "playing" {
		return PlaceResult{}
	}
	member, role, ok := r.findMemberLocked(uuid)
	if !ok || role == "spectator" || member.isOffline {
		return PlaceResult{}
	}
	color := r.hostColor
	if role == "opponent" {
		color = r.opponentColor
	}
	if r.turn != color || row < 0 || row >= 15 || col < 0 || col >= 15 {
		return PlaceResult{}
	}
	for _, move := range r.history {
		if move.Row == row && move.Col == col {
			return PlaceResult{}
		}
	}

	r.history = append(r.history, Move{Row: row, Col: col, Player: color})
	r.retractRequester = ""
	r.retractRequesterName = ""
	result := PlaceResult{Placed: true}
	if winCoords := r.checkWinLocked(row, col, color); winCoords != nil {
		r.finishMatchLocked(color, winCoords)
		result.Finished = true
		result.Winner = color
		result.WinnerName = r.playerNameByColorLocked(color)
		return result
	}
	if len(r.history) == 225 {
		r.finishMatchLocked("draw", nil)
		result.Finished = true
		result.Winner = "draw"
		return result
	}
	if r.turn == "black" {
		r.turn = "white"
	} else {
		r.turn = "black"
	}
	return result
}

func (r *Room) RequestRetract(uuid string) RetractRequestResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	member, role, ok := r.findMemberLocked(uuid)
	if r.status != "playing" || len(r.history) == 0 || !ok || role == "spectator" || member.isOffline {
		return RetractRequestResult{}
	}
	if r.retractRequester != "" {
		return RetractRequestResult{}
	}
	if last, exists := r.lastRetractTimeByPlayer[uuid]; exists && time.Since(last) < 5*time.Second {
		return RetractRequestResult{}
	}
	r.retractRequester = uuid
	r.retractRequesterName = member.player.Name
	r.lastRetractTimeByPlayer[uuid] = time.Now()
	return RetractRequestResult{Requested: true, RequesterName: member.player.Name}
}

func (r *Room) HandleRetractResponse(uuid string, agree bool) RetractResponseResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	member, role, ok := r.findMemberLocked(uuid)
	if r.status != "playing" || r.retractRequester == "" || !ok || role == "spectator" || member.isOffline || r.retractRequester == uuid {
		return RetractResponseResult{}
	}
	requesterName := r.retractRequesterName
	r.retractRequester = ""
	r.retractRequesterName = ""
	result := RetractResponseResult{
		Valid:         true,
		Agreed:        agree,
		ResponderName: member.player.Name,
		RequesterName: requesterName,
	}
	if !agree || len(r.history) == 0 {
		return result
	}
	lastMove := r.history[len(r.history)-1]
	r.history = r.history[:len(r.history)-1]
	r.turn = lastMove.Player
	return result
}

func (r *Room) Resign(uuid string) ResignResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	member, role, ok := r.findMemberLocked(uuid)
	if r.status != "playing" || !ok || role == "spectator" || member.isOffline {
		return ResignResult{}
	}
	resigningColor := r.hostColor
	if role == "opponent" {
		resigningColor = r.opponentColor
	}
	winner := "black"
	if resigningColor == "black" {
		winner = "white"
	}
	r.finishMatchLocked(winner, nil)
	return ResignResult{Resigned: true, PlayerName: member.player.Name}
}

func (r *Room) UpdateConfig(uuid string, config RoomConfig) ConfigUpdateResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.host == nil || r.host.player.UUID != uuid || !ValidateRoomConfig(config) {
		return ConfigUpdateResult{}
	}
	wasAutoJoin := r.config.AutoJoinSpectator
	r.config = config
	result := ConfigUpdateResult{Updated: true}
	if !wasAutoJoin && config.AutoJoinSpectator && r.status == "waiting" && r.opponent == nil && len(r.spectators) > 0 {
		r.opponent = r.spectators[0]
		r.opponent.isReady = false
		r.spectators = r.spectators[1:]
		result.PromotedName = r.opponent.player.Name
	}
	return result
}

func (r *Room) ClaimSeat(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != "waiting" || r.opponent != nil {
		return false
	}
	for i, spectator := range r.spectators {
		if spectator.player.UUID == uuid && !spectator.isOffline {
			r.opponent = spectator
			r.opponent.isReady = false
			r.spectators = append(r.spectators[:i], r.spectators[i+1:]...)
			return true
		}
	}
	return false
}

func (r *Room) ChatSender(uuid string) (*Player, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.config.DisableChat {
		return nil, ErrorChatDisabled
	}
	member, role, ok := r.findMemberLocked(uuid)
	if !ok {
		return nil, ErrorNotRoomMember
	}
	if role == "spectator" {
		return nil, ErrorSpectatorChat
	}
	if member.isOffline {
		return nil, ErrorPermissionDenied
	}
	return member.player, ""
}

func (r *Room) findMemberLocked(uuid string) (*roomMember, string, bool) {
	if r.host != nil && r.host.player.UUID == uuid {
		return r.host, "host", true
	}
	if r.opponent != nil && r.opponent.player.UUID == uuid {
		return r.opponent, "opponent", true
	}
	for _, spectator := range r.spectators {
		if spectator.player.UUID == uuid {
			return spectator, "spectator", true
		}
	}
	return nil, "", false
}

func (r *Room) handleVacancyLocked() {
	if r.host == nil && r.opponent != nil {
		r.host = r.opponent
		r.opponent = nil
	}
	if r.host == nil && len(r.spectators) > 0 {
		r.host = r.spectators[0]
		r.host.isReady = false
		r.spectators = r.spectators[1:]
	}
	if r.opponent == nil && r.config.AutoJoinSpectator && len(r.spectators) > 0 {
		r.opponent = r.spectators[0]
		r.opponent.isReady = false
		r.spectators = r.spectators[1:]
	}
}

func (r *Room) resetMatchLocked() {
	r.status = "waiting"
	r.history = make([]Move, 0)
	r.winner = ""
	r.winningLine = nil
	r.retractRequester = ""
	r.retractRequesterName = ""
	if r.host != nil {
		r.host.isReady = false
	}
	if r.opponent != nil {
		r.opponent.isReady = false
	}
}

func (r *Room) finishMatchLocked(winner string, winningLine []Coord) {
	r.status = "waiting"
	r.winner = winner
	r.winningLine = winningLine
	r.retractRequester = ""
	r.retractRequesterName = ""
	if r.host != nil {
		r.host.isReady = false
	}
	if r.opponent != nil {
		r.opponent.isReady = false
	}
}

func (r *Room) isEmptyLocked() bool {
	return r.host == nil && r.opponent == nil && len(r.spectators) == 0
}

func (r *Room) playerNameByColorLocked(color string) string {
	if r.host != nil && r.hostColor == color {
		return r.host.player.Name
	}
	if r.opponent != nil && r.opponentColor == color {
		return r.opponent.player.Name
	}
	return ""
}

func (r *Room) checkWinLocked(row, col int, color string) []Coord {
	board := [15][15]string{}
	for _, move := range r.history {
		board[move.Row][move.Col] = move.Player
	}
	directions := [][][2]int{
		{{0, 1}, {0, -1}},
		{{1, 0}, {-1, 0}},
		{{1, 1}, {-1, -1}},
		{{1, -1}, {-1, 1}},
	}
	for _, direction := range directions {
		coords := []Coord{{Row: row, Col: col}}
		for _, step := range direction {
			nextRow, nextCol := row+step[0], col+step[1]
			for nextRow >= 0 && nextRow < 15 && nextCol >= 0 && nextCol < 15 && board[nextRow][nextCol] == color {
				coords = append(coords, Coord{Row: nextRow, Col: nextCol})
				nextRow += step[0]
				nextCol += step[1]
			}
		}
		if len(coords) >= 5 {
			return coords
		}
	}
	return nil
}
