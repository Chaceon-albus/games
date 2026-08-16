package gomoku

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRoomSnapshotProtectsCredentialsAndIdentifiesViewer(t *testing.T) {
	host := testPlayer(1, "same-name")
	opponent := testPlayer(2, "same-name")
	room := NewRoom("000001", "privacy", host)
	if _, err := room.AddPlayer(opponent); err != nil {
		t.Fatalf("add opponent: %v", err)
	}
	if !room.ToggleReady(host.UUID).Changed || !room.ToggleReady(opponent.UUID).Started {
		t.Fatal("failed to start game")
	}
	if !room.PlaceStone(host.UUID, 0, 0).Placed {
		t.Fatal("failed to place initial stone")
	}
	if !room.RequestRetract(host.UUID).Requested {
		t.Fatal("failed to request retract")
	}

	hostView := room.Snapshot(host.UUID)
	opponentView := room.Snapshot(opponent.UUID)
	if hostView.Self.Role != "host" || hostView.Self.PlayerID != host.PublicID {
		t.Fatalf("unexpected host identity: %+v", hostView.Self)
	}
	if opponentView.Self.Role != "opponent" || opponentView.Self.PlayerID != opponent.PublicID {
		t.Fatalf("unexpected opponent identity: %+v", opponentView.Self)
	}
	if !hostView.RetractRequestedBySelf || opponentView.RetractRequestedBySelf {
		t.Fatal("retract ownership was not personalized")
	}

	payload, err := json.Marshal(opponentView)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(payload), host.UUID) || strings.Contains(string(payload), opponent.UUID) {
		t.Fatalf("private credential leaked in room state: %s", payload)
	}
}

func TestRoomAllowsExactlyMaximumSpectators(t *testing.T) {
	room := NewRoom("000002", "capacity", testPlayer(1, "host"))
	if _, err := room.AddPlayer(testPlayer(2, "opponent")); err != nil {
		t.Fatalf("add opponent: %v", err)
	}
	for index := 0; index < MaxSpectatorsPerRoom; index++ {
		role, err := room.AddPlayer(testPlayer(index+3, fmt.Sprintf("spectator-%d", index)))
		if err != nil || role != "spectator" {
			t.Fatalf("add spectator %d: role=%q err=%v", index, role, err)
		}
	}
	if _, err := room.AddPlayer(testPlayer(MaxSpectatorsPerRoom+3, "overflow")); err != ErrSpectatorCapacity {
		t.Fatalf("expected capacity error, got %v", err)
	}
	if got := len(room.Snapshot(testPlayer(9999, "viewer").UUID).Spectators); got != MaxSpectatorsPerRoom {
		t.Fatalf("spectator count = %d, want %d", got, MaxSpectatorsPerRoom)
	}
}

func TestRoomVacancyCanBeClaimedOrAutomaticallyFilled(t *testing.T) {
	host := testPlayer(1, "host")
	opponent := testPlayer(2, "opponent")
	spectator := testPlayer(3, "spectator")
	room := NewRoom("000003", "claim", host)
	_, _ = room.AddPlayer(opponent)
	_, _ = room.AddPlayer(spectator)
	room.RemovePlayer(opponent.UUID)
	if !room.ClaimSeat(spectator.UUID) {
		t.Fatal("spectator could not claim an open opponent seat")
	}
	if view := room.Snapshot(spectator.UUID); view.Self.Role != "opponent" {
		t.Fatalf("claimed role = %q, want opponent", view.Self.Role)
	}

	autoRoom := NewRoom("000004", "auto", host)
	autoOpponent := testPlayer(4, "auto-opponent")
	autoSpectator := testPlayer(5, "auto-spectator")
	_, _ = autoRoom.AddPlayer(autoOpponent)
	_, _ = autoRoom.AddPlayer(autoSpectator)
	autoRoom.RemovePlayer(autoOpponent.UUID)
	result := autoRoom.UpdateConfig(host.UUID, RoomConfig{AutoJoinSpectator: true, ColorMode: "alternating"})
	if result.PromotedName != autoSpectator.Name {
		t.Fatalf("promoted player = %q, want %q", result.PromotedName, autoSpectator.Name)
	}
}

func TestNewPlayerFillsVacancyAheadOfSpectatorQueue(t *testing.T) {
	host := testPlayer(1, "host")
	opponent := testPlayer(2, "opponent")
	spectator := testPlayer(3, "spectator")
	newPlayer := testPlayer(4, "new-player")
	room := NewRoom("000005", "vacancy", host)
	_, _ = room.AddPlayer(opponent)
	_, _ = room.AddPlayer(spectator)
	room.RemovePlayer(opponent.UUID)
	role, err := room.AddPlayer(newPlayer)
	if err != nil || role != "opponent" {
		t.Fatalf("new player role=%q err=%v, want opponent", role, err)
	}
}

func TestChatPermissionsAreServerAuthoritative(t *testing.T) {
	host := testPlayer(1, "host")
	opponent := testPlayer(2, "opponent")
	spectator := testPlayer(3, "spectator")
	room := NewRoom("000006", "chat", host)
	_, _ = room.AddPlayer(opponent)
	_, _ = room.AddPlayer(spectator)

	if _, code := room.ChatSender(spectator.UUID); code != ErrorSpectatorChat {
		t.Fatalf("spectator chat code = %q, want %q", code, ErrorSpectatorChat)
	}
	room.UpdateConfig(host.UUID, RoomConfig{DisableChat: true, ColorMode: "alternating"})
	if _, code := room.ChatSender(host.UUID); code != ErrorChatDisabled {
		t.Fatalf("disabled chat code = %q, want %q", code, ErrorChatDisabled)
	}
}

func TestRoomSupportsConcurrentSnapshotsAndConfigUpdates(t *testing.T) {
	host := testPlayer(1, "host")
	room := NewRoom("000007", "concurrent", host)
	done := make(chan struct{}, 100)
	for index := 0; index < 50; index++ {
		go func() {
			_ = room.Snapshot(host.UUID)
			done <- struct{}{}
		}()
		go func(disableChat bool) {
			room.UpdateConfig(host.UUID, RoomConfig{DisableChat: disableChat, ColorMode: "alternating"})
			done <- struct{}{}
		}(index%2 == 0)
	}
	for index := 0; index < 100; index++ {
		<-done
	}
}

func testPlayer(index int, name string) *Player {
	return &Player{
		UUID:     fmt.Sprintf("private-%d", index),
		PublicID: fmt.Sprintf("public-%d", index),
		Name:     name,
	}
}
