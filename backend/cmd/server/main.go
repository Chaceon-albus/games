package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Chaceon-albus/games/backend/internal/gomoku"
	"github.com/gin-gonic/gin"
)

func main() {

	// ==========================================================
	// 1. Command-Line Flags Parsing
	// ==========================================================
	// Define flags for network binding configuration
	hostPtr := flag.String("host", "0.0.0.0", "The IP address to bind the server to")
	portPtr := flag.Int("port", 8080, "The port number to listen on")
	levelPtr := flag.String("level", "info", "Log output severity level (debug, info, warn, error)")

	// Execute the command-line parsing
	flag.Parse()

	var programLevel slog.Level

	// Parse the string input into the standard structured slog.Level
	switch strings.ToLower(strings.TrimSpace(*levelPtr)) {
	case "debug":
		programLevel = slog.LevelDebug
	case "info":
		programLevel = slog.LevelInfo
	case "warn":
		programLevel = slog.LevelWarn
	case "error":
		programLevel = slog.LevelError
	default:
		// Fallback safely to INFO if an invalid severity name was provided
		programLevel = slog.LevelInfo
		// Temporarily use an ephemeral logger to announce the parsing fallback
		fallbackLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		fallbackLogger.Warn("Invalid log level provided, falling back to default level",
			slog.String("received", *levelPtr),
			slog.String("fallback", "info"),
		)
	}

	// ==========================================================
	// 2. Logger Initialization
	// ==========================================================
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: programLevel,
	}))
	slog.SetDefault(logger)

	// Combine host and port into a standard network address string (e.g., "0.0.0.0:8080")
	addr := fmt.Sprintf("%s:%d", *hostPtr, *portPtr)

	// ==========================================================
	// 3. Game Modules Initialization
	// ==========================================================
	// Initialize the individual game instances and fetch their Melody handlers
	slog.Info("Loading game modules...")
	gomokuMelody := gomoku.Init()

	// TODO: Add other game modules initialization here in the future
	// chessMelody := chess.InitChess()

	// ==========================================================
	// 4. Gin Router & Routes Setup
	// ==========================================================
	// Set Gin to release mode if running in production to disable verbose debugging logs
	// gin.SetMode(gin.ReleaseMode)

	setGinMode()
	r := gin.Default()

	// WebSocket endpoint for Gomoku.
	r.GET("/ws/gomoku", func(c *gin.Context) {
		slog.Debug("Incoming WebSocket upgrade request for Gomoku",
			slog.String("remote_ip", c.ClientIP()),
		)
		gomokuMelody.HandleRequest(c.Writer, c.Request)
	})

	// RESTful API Routing Group for metadata, leaderboards, and general info
	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "healthy", "message": "pong"})
		})
	}

	// ==========================================================
	// 5. Server Startup
	// ==========================================================
	slog.Info("Game server is running successfully",
		slog.String("target_address", addr),
	)

	// Start listening and serving HTTP/WebSocket connections
	if err := r.Run(addr); err != nil {
		slog.Error("Fatal error encountered while running the server",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
}
