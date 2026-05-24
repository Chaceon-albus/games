//go:build release

package main

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

func setGinMode() {
	slog.Info("Running server in PRODUCTION mode (Gin: release)")

	// Force Gin into silent release mode, eliminating all debug spam
	gin.SetMode(gin.ReleaseMode)
}
