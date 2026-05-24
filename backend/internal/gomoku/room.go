package gomoku

import (
	"math/rand"
	"sync"
	"time"
)

type RoomConfig struct {
	AutoJoinSpectator bool   `json:"autoJoinSpectator"`
	DisableChat       bool   `json:"disableChat"`
	ColorMode         string `json:"colorMode"` // "alternating" or "random"
}

type Move struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Player string `json:"player"` // "black" or "white"
}

type Coord struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type Room struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Status               string               `json:"status"`   // "waiting" or "playing"
	Host                 *Player              `json:"host"`     // First player to join
	Opponent             *Player              `json:"opponent"` // Second player to join, or nil
	Spectators           []*Player            `json:"spectators"`
	Config               RoomConfig           `json:"config"`
	History              []Move               `json:"history"`
	Turn                 string               `json:"turn"`   // "black" or "white"
	Winner               string               `json:"winner"` // "black", "white", "draw", or ""
	WinningLine          []Coord              `json:"winningLine"`
	HostColor            string               `json:"hostColor"`     // "black" or "white"
	OpponentColor        string               `json:"opponentColor"` // "black" or "white"
	ConsecutiveGames     int                  `json:"consecutiveGames"`
	RetractRequester     string               `json:"retractRequester"`     // UUID of player requesting retract
	RetractRequesterName string               `json:"retractRequesterName"` // Public display name of retract requester
	LastRetractTime      map[string]time.Time `json:"-"`
	mu                   sync.Mutex           `json:"-"`
}

// NewRoom creates a new Room with standard settings
func NewRoom(id, name string, creator *Player) *Room {
	return &Room{
		ID:     id,
		Name:   name,
		Status: "waiting",
		Host:   creator,
		Config: RoomConfig{
			AutoJoinSpectator: false,
			DisableChat:       false,
			ColorMode:         "alternating",
		},
		History:          make([]Move, 0),
		Spectators:       make([]*Player, 0),
		HostColor:        "black",
		OpponentColor:    "white",
		ConsecutiveGames: 0,
		LastRetractTime:  make(map[string]time.Time),
	}
}

// AddPlayer adds a player to the room, assigning roles based on availability
func (r *Room) AddPlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure player starts clean
	p.IsReady = false
	p.IsOffline = false

	if r.Host == nil {
		r.Host = p
	} else if r.Opponent == nil && len(r.Spectators) == 0 {
		r.Opponent = p
	} else {
		// Add to spectators list (ordered by entry time)
		r.Spectators = append(r.Spectators, p)
	}
}

// RemovePlayer handles a player leaving or being disconnected permanently
func (r *Room) RemovePlayer(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	wasHost := r.Host != nil && r.Host.UUID == uuid
	wasOpponent := r.Opponent != nil && r.Opponent.UUID == uuid

	if wasHost {
		r.Host = nil
	} else if wasOpponent {
		r.Opponent = nil
	} else {
		// Remove from spectators
		found := -1
		for i, spec := range r.Spectators {
			if spec.UUID == uuid {
				found = i
				break
			}
		}
		if found != -1 {
			r.Spectators = append(r.Spectators[:found], r.Spectators[found+1:]...)
		}
		return r.Host == nil && r.Opponent == nil && len(r.Spectators) == 0
	}

	// Game resets if an active player leaves
	if r.Status == "playing" && (wasHost || wasOpponent) {
		r.Status = "waiting"
		r.History = make([]Move, 0)
		r.Winner = ""
		r.WinningLine = nil
		r.RetractRequester = ""
		r.RetractRequesterName = ""
		if r.Host != nil {
			r.Host.IsReady = false
		}
		if r.Opponent != nil {
			r.Opponent.IsReady = false
		}
	}

	// Handle role shifting / spectator auto-join
	r.handleVacancy()

	// Return true if room is completely empty and should be deleted
	return r.Host == nil && r.Opponent == nil && len(r.Spectators) == 0
}

// HasPlayer checks if a player is in the room as an active player or spectator
func (r *Room) HasPlayer(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Host != nil && r.Host.UUID == uuid {
		return true
	}
	if r.Opponent != nil && r.Opponent.UUID == uuid {
		return true
	}
	for _, spec := range r.Spectators {
		if spec.UUID == uuid {
			return true
		}
	}
	return false
}

// handleVacancy handles promoting other players/spectators when slots open up
func (r *Room) handleVacancy() {
	// 1. If Host left but Opponent is present, promote Opponent to Host
	if r.Host == nil && r.Opponent != nil {
		r.Host = r.Opponent
		r.Opponent = nil
	}

	// 2. If Opponent is empty (due to promotion or leaving) and Config.AutoJoinSpectator is active
	// and there is at least one spectator, promote the first spectator to Opponent.
	if r.Opponent == nil && r.Config.AutoJoinSpectator && len(r.Spectators) > 0 {
		r.Opponent = r.Spectators[0]
		r.Opponent.IsReady = false
		r.Spectators = r.Spectators[1:]
	}

	// 3. If Host is empty and there are spectators (e.g. no opponent), promote first spectator to Host
	if r.Host == nil && len(r.Spectators) > 0 {
		r.Host = r.Spectators[0]
		r.Host.IsReady = false
		r.Spectators = r.Spectators[1:]

		// If there is another spectator and auto-join is on, fill the opponent slot
		if r.Opponent == nil && r.Config.AutoJoinSpectator && len(r.Spectators) > 0 {
			r.Opponent = r.Spectators[0]
			r.Opponent.IsReady = false
			r.Spectators = r.Spectators[1:]
		}
	}
}

// SetPlayerOffline marks a player as disconnected but keeps them in the room
func (r *Room) SetPlayerOffline(uuid string, offline bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Host != nil && r.Host.UUID == uuid {
		r.Host.IsOffline = offline
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		r.Opponent.IsOffline = offline
	} else {
		for _, spec := range r.Spectators {
			if spec.UUID == uuid {
				spec.IsOffline = offline
			}
		}
	}
}

// ToggleReady toggles ready state for host or opponent, starting the game if both ready
func (r *Room) ToggleReady(uuid string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status == "playing" {
		return
	}

	var p *Player
	if r.Host != nil && r.Host.UUID == uuid {
		p = r.Host
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		p = r.Opponent
	}

	if p == nil {
		return
	}

	p.IsReady = !p.IsReady

	// Start game if both players are ready and exist
	if r.Host != nil && r.Host.IsReady && r.Opponent != nil && r.Opponent.IsReady {
		r.Status = "playing"
		r.History = make([]Move, 0)
		r.Winner = ""
		r.WinningLine = nil
		r.Turn = "black"
		r.RetractRequester = ""
		r.RetractRequesterName = ""

		// Determine colors
		if r.Config.ColorMode == "random" {
			if rand.Intn(2) == 0 {
				r.HostColor = "black"
				r.OpponentColor = "white"
			} else {
				r.HostColor = "white"
				r.OpponentColor = "black"
			}
		} else {
			// Alternating mode
			if r.ConsecutiveGames == 0 {
				r.HostColor = "black"
				r.OpponentColor = "white"
			} else {
				// Swap colors
				if r.HostColor == "black" {
					r.HostColor = "white"
					r.OpponentColor = "black"
				} else {
					r.HostColor = "black"
					r.OpponentColor = "white"
				}
			}
		}
		r.ConsecutiveGames++
	}
}

// PlaceStone validates and executes a stone placement
func (r *Room) PlaceStone(uuid string, row, col int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != "playing" {
		return false
	}

	// 1. Identify player color
	var color string
	if r.Host != nil && r.Host.UUID == uuid {
		color = r.HostColor
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		color = r.OpponentColor
	} else {
		return false // Spectator cannot play
	}

	// 2. Validate turn
	if r.Turn != color {
		return false
	}

	// 3. Validate coordinates and board cell occupancy
	if row < 0 || row >= 15 || col < 0 || col >= 15 {
		return false
	}
	for _, m := range r.History {
		if m.Row == row && m.Col == col {
			return false // Already occupied
		}
	}

	// 4. Place move
	move := Move{Row: row, Col: col, Player: color}
	r.History = append(r.History, move)
	r.RetractRequester = "" // Clear any pending retract on new move
	r.RetractRequesterName = ""

	// 5. Check win
	winCoords := r.checkWin(row, col, color)
	if winCoords != nil {
		r.Status = "waiting"
		r.Winner = color
		r.WinningLine = winCoords
		if r.Host != nil {
			r.Host.IsReady = false
		}
		if r.Opponent != nil {
			r.Opponent.IsReady = false
		}
		return true
	}

	// 6. Check draw
	if len(r.History) == 225 {
		r.Status = "waiting"
		r.Winner = "draw"
		if r.Host != nil {
			r.Host.IsReady = false
		}
		if r.Opponent != nil {
			r.Opponent.IsReady = false
		}
		return true
	}

	// 7. Alternate turn
	if r.Turn == "black" {
		r.Turn = "white"
	} else {
		r.Turn = "black"
	}

	return false
}

// RequestRetract sets a pending retract request if eligible
func (r *Room) RequestRetract(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Must be playing and have at least one move in history
	if r.Status != "playing" || len(r.History) == 0 {
		return false
	}

	// Only active players can request retract
	isHost := r.Host != nil && r.Host.UUID == uuid
	isOpponent := r.Opponent != nil && r.Opponent.UUID == uuid
	if !isHost && !isOpponent {
		return false
	}

	// 1. Prevent duplicate active retract requests
	if r.RetractRequester != "" {
		return false
	}

	// 2. Enforce 5-second cooldown per player
	if r.LastRetractTime == nil {
		r.LastRetractTime = make(map[string]time.Time)
	}
	lastTime, exists := r.LastRetractTime[uuid]
	if exists && time.Since(lastTime) < 5*time.Second {
		return false
	}

	// Register requester and update cooldown timestamp
	r.RetractRequester = uuid
	r.LastRetractTime[uuid] = time.Now()

	if r.Host != nil && r.Host.UUID == uuid {
		r.RetractRequesterName = r.Host.Name
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		r.RetractRequesterName = r.Opponent.Name
	}
	return true
}

// HandleRetractResponse processes the opponent's response to a retract request
func (r *Room) HandleRetractResponse(approverUUID string, agree bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.RetractRequester == "" {
		return false
	}

	// Validate that the responder is the other active player
	isHost := r.Host != nil && r.Host.UUID == approverUUID
	isOpponent := r.Opponent != nil && r.Opponent.UUID == approverUUID
	if !isHost && !isOpponent {
		return false
	}
	if r.RetractRequester == approverUUID {
		return false // Cannot approve own request
	}

	defer func() {
		r.RetractRequester = "" // Always clear request
		r.RetractRequesterName = ""
	}()

	if !agree {
		return false
	}

	// Apply retract: pop last move
	lastIndex := len(r.History) - 1
	lastMove := r.History[lastIndex]
	r.History = r.History[:lastIndex]

	// Reset turn to the popped player's turn
	r.Turn = lastMove.Player
	return true
}

// Resign handles a player resigning from the match
func (r *Room) Resign(uuid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != "playing" {
		return false
	}

	var resigningColor string
	if r.Host != nil && r.Host.UUID == uuid {
		resigningColor = r.HostColor
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		resigningColor = r.OpponentColor
	} else {
		return false
	}

	r.Status = "waiting"
	if resigningColor == "black" {
		r.Winner = "white"
	} else {
		r.Winner = "black"
	}

	if r.Host != nil {
		r.Host.IsReady = false
	}
	if r.Opponent != nil {
		r.Opponent.IsReady = false
	}
	r.RetractRequester = ""
	r.RetractRequesterName = ""

	return true
}

// SwitchRole handles moving a player between host, opponent, and spectator
func (r *Room) SwitchRole(uuid string, targetRole string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Cannot switch roles mid-game
	if r.Status == "playing" {
		return false
	}

	// Find the player
	var targetPlayer *Player
	currentRole := ""

	if r.Host != nil && r.Host.UUID == uuid {
		targetPlayer = r.Host
		currentRole = "host"
	} else if r.Opponent != nil && r.Opponent.UUID == uuid {
		targetPlayer = r.Opponent
		currentRole = "opponent"
	} else {
		for i, spec := range r.Spectators {
			if spec.UUID == uuid {
				targetPlayer = spec
				currentRole = "spectator"
				// Temporarily remove from spectator list
				r.Spectators = append(r.Spectators[:i], r.Spectators[i+1:]...)
				break
			}
		}
	}

	if targetPlayer == nil {
		return false
	}

	success := false
	targetPlayer.IsReady = false

	switch targetRole {
	case "host", "black": // Match legacy "black" simulation role
		if r.Host == nil {
			// Vacuum previous slot
			if currentRole == "opponent" {
				r.Opponent = nil
			}
			r.Host = targetPlayer
			success = true
		}
	case "opponent", "white": // Match legacy "white" simulation role
		if r.Opponent == nil {
			// Vacuum previous slot
			if currentRole == "host" {
				r.Host = nil
			}
			r.Opponent = targetPlayer
			success = true
		}
	case "spectator":
		switch currentRole {
		case "host":
			r.Host = nil
		case "opponent":
			r.Opponent = nil
		}
		r.Spectators = append(r.Spectators, targetPlayer)
		success = true
	}

	// Revert spectator remove if failed
	if !success && currentRole == "spectator" {
		r.Spectators = append(r.Spectators, targetPlayer)
	}

	// Re-balance slots in case players left slots empty
	r.handleVacancy()

	return success
}

// UpdateConfig updates the room settings
func (r *Room) UpdateConfig(uuid string, config RoomConfig) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Only host can edit config
	if r.Host == nil || r.Host.UUID != uuid {
		return false
	}

	r.Config = config
	return true
}

func (r *Room) checkWin(row, col int, color string) []Coord {
	board := [15][15]string{}
	for _, m := range r.History {
		board[m.Row][m.Col] = m.Player
	}

	directions := [][][2]int{
		{{0, 1}, {0, -1}},  // Horizontal
		{{1, 0}, {-1, 0}},  // Vertical
		{{1, 1}, {-1, -1}}, // Diagonal \
		{{1, -1}, {-1, 1}}, // Counter-diagonal /
	}

	for _, dir := range directions {
		coords := []Coord{{Row: row, Col: col}}
		for _, step := range dir {
			dr, dc := step[0], step[1]
			nr, nc := row+dr, col+dc
			for nr >= 0 && nr < 15 && nc >= 0 && nc < 15 && board[nr][nc] == color {
				coords = append(coords, Coord{Row: nr, Col: nc})
				nr += dr
				nc += dc
			}
		}
		if len(coords) >= 5 {
			return coords
		}
	}
	return nil
}
