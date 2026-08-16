package gomoku

import (
	"testing"
	"time"

	"github.com/olahol/melody"
)

func TestStaleDisconnectDoesNotRemoveCurrentSession(t *testing.T) {
	manager := newManager(melody.New())
	player, err := manager.registry.Register("player")
	if err != nil {
		t.Fatalf("register player: %v", err)
	}
	room := NewRoom("100001", "reconnect", player)
	oldSession := &melody.Session{}
	newSession := &melody.Session{}
	manager.rooms[room.ID()] = room
	manager.memberships[player.UUID] = room.ID()
	manager.connectionSerial[player.UUID] = 2
	manager.activeSessions[player.UUID] = newSession
	manager.sessions[oldSession] = &sessionBinding{uuid: player.UUID, generation: 1, limiter: newSessionRateLimiter()}
	manager.sessions[newSession] = &sessionBinding{uuid: player.UUID, generation: 2, limiter: newSessionRateLimiter()}

	manager.handleDisconnect(oldSession)

	if manager.activeSessions[player.UUID] != newSession {
		t.Fatal("stale disconnect replaced the current session")
	}
	if manager.memberships[player.UUID] != room.ID() || !room.HasPlayer(player.UUID) {
		t.Fatal("stale disconnect removed current room membership")
	}
}

func TestDisconnectTimeoutCompletesWithoutManagerDeadlock(t *testing.T) {
	manager := newManager(melody.New())
	host, err := manager.registry.Register("host")
	if err != nil {
		t.Fatalf("register host: %v", err)
	}
	opponent, err := manager.registry.Register("opponent")
	if err != nil {
		t.Fatalf("register opponent: %v", err)
	}
	room := NewRoom("100002", "timeout", host)
	_, _ = room.AddPlayer(opponent)
	room.ToggleReady(host.UUID)
	room.ToggleReady(opponent.UUID)
	if !room.DisconnectPlayer(host.UUID).NeedsGrace {
		t.Fatal("playing host did not enter reconnect grace period")
	}
	manager.rooms[room.ID()] = room
	manager.memberships[host.UUID] = room.ID()
	manager.memberships[opponent.UUID] = room.ID()
	manager.connectionSerial[host.UUID] = 1

	done := make(chan struct{})
	go func() {
		manager.handleDisconnectTimeout(host.UUID, 1, room.ID())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect timeout deadlocked")
	}
	if _, exists := manager.memberships[host.UUID]; exists {
		t.Fatal("timed out player still has room membership")
	}
	if rooms := manager.getLobbyRooms(); len(rooms) != 1 {
		t.Fatalf("lobby room count = %d, want 1", len(rooms))
	}
}

func TestManagerEnforcesSingleRoomMembership(t *testing.T) {
	manager := newManager(melody.New())
	player, err := manager.registry.Register("player")
	if err != nil {
		t.Fatalf("register player: %v", err)
	}
	first, code := manager.createRoom(player.UUID, "first", RoomConfig{ColorMode: "alternating"}, player)
	if code != "" {
		t.Fatalf("create first room: %s", code)
	}
	if _, code = manager.createRoom(player.UUID, "second", RoomConfig{ColorMode: "alternating"}, player); code != ErrorAlreadyInRoom {
		t.Fatalf("second room code = %q, want %q", code, ErrorAlreadyInRoom)
	}
	if got := manager.memberships[player.UUID]; got != first.ID() {
		t.Fatalf("membership = %q, want %q", got, first.ID())
	}
}

func TestSessionRateLimiterRestrictsChatBurst(t *testing.T) {
	limiter := newSessionRateLimiter()
	if !limiter.Allow("send_chat") {
		t.Fatal("first chat message was rejected")
	}
	if limiter.Allow("send_chat") {
		t.Fatal("second immediate chat message was accepted")
	}
}
