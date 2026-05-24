package gomoku

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/olahol/melody"
)

// Player represents an active or temporarily disconnected player
type Player struct {
	UUID         string          `json:"-"`       // Kept private: never sent to other players
	Name         string          `json:"name"`    // Display nickname
	Avatar       string          `json:"avatar"`  // Placeholder avatar (empty)
	IsReady      bool            `json:"isReady"`
	IsOffline    bool            `json:"isOffline"`
	DisconnectAt time.Time       `json:"-"`
	Session      *melody.Session `json:"-"`       // Reference to active WS session
}

// PlayerRegistry manages active players in memory in a thread-safe manner
type PlayerRegistry struct {
	mu      sync.RWMutex
	players map[string]*Player
}

// NewPlayerRegistry creates an initialized PlayerRegistry
func NewPlayerRegistry() *PlayerRegistry {
	return &PlayerRegistry{
		players: make(map[string]*Player),
	}
}

// Register creates a new Player and returns it
func (pr *PlayerRegistry) Register(name string) (*Player, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	uuid, err := generateUUID()
	if err != nil {
		return nil, err
	}

	player := &Player{
		UUID: uuid,
		Name: name,
	}
	pr.players[uuid] = player
	return player, nil
}

// Get retrieves a player by UUID
func (pr *PlayerRegistry) Get(uuid string) (*Player, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	player, ok := pr.players[uuid]
	return player, ok
}

// Remove deletes a player from the registry
func (pr *PlayerRegistry) Remove(uuid string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.players, uuid)
}

// Helper to generate a random 16-byte UUID string
func generateUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
