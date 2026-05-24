package gomoku

import (
	"log/slog"

	"github.com/olahol/melody"
)

// Init initializes the Gomoku game engine module and registers
// reactive WebSocket lifecycle hooks. It returns an active Melody pointer.
func Init() *melody.Melody {
	m := melody.New()

	// HandleConnect fires instantly when a new player upgrades to a WebSocket connection
	m.HandleConnect(func(s *melody.Session) {
		slog.Info("Gomoku: New player connected",
			slog.String("remote_address", s.Request.RemoteAddr),
		)
	})

	// HandleDisconnect fires when a client loses connection, closes the browser, or leaves
	m.HandleDisconnect(func(s *melody.Session) {
		slog.Info("Gomoku: Player disconnected",
			slog.String("remote_address", s.Request.RemoteAddr),
		)
	})

	// HandleMessage triggers whenever a payload is received from a client.
	// This core MVP block behaves as a real-time echo broadcast chamber.
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		slog.Debug("Gomoku: Received payload",
			slog.String("payload", string(msg)),
		)

		// Broadcast incoming data to EVERY client currently connected to the Gomoku
		if err := m.Broadcast(msg); err != nil {
			slog.Error("Gomoku: Failed to broadcast message",
				slog.String("error", err.Error()),
			)
		}
	})

	// HandleError catches low-level socket protocol exceptions gracefully
	m.HandleError(func(s *melody.Session, err error) {
		slog.Warn("Gomoku: Connection connection error detected",
			slog.String("error", err.Error()),
		)
	})

	return m
}
