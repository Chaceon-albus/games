package gomoku

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Player contains immutable identity data. UUID is a private session credential,
// while PublicID is safe to expose to other clients.
type Player struct {
	UUID     string
	PublicID string
	Name     string
	Avatar   string
}

// PlayerRegistry manages registered players in memory.
type PlayerRegistry struct {
	mu      sync.RWMutex
	players map[string]*Player
}

// NewPlayerRegistry creates an initialized PlayerRegistry.
func NewPlayerRegistry() *PlayerRegistry {
	return &PlayerRegistry{players: make(map[string]*Player)}
}

// Register creates a player with separate private and public identifiers.
func (pr *PlayerRegistry) Register(name string) (*Player, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	for {
		uuid, err := generateOpaqueID(16)
		if err != nil {
			return nil, err
		}
		if _, exists := pr.players[uuid]; exists {
			continue
		}

		publicID, err := generateOpaqueID(12)
		if err != nil {
			return nil, err
		}

		player := &Player{
			UUID:     uuid,
			PublicID: "player_" + publicID,
			Name:     name,
		}
		pr.players[uuid] = player
		return player, nil
	}
}

// Get retrieves a player by its private session credential.
func (pr *PlayerRegistry) Get(uuid string) (*Player, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	player, ok := pr.players[uuid]
	return player, ok
}

// Remove deletes a player from the registry.
func (pr *PlayerRegistry) Remove(uuid string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.players, uuid)
}

func generateOpaqueID(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
