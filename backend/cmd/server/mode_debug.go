//go:build !release

package main

import "log/slog"

func setGinMode() {
	slog.Info("Running server in LOCAL DEVELOPMENT mode (Gin: debug)")
	// Do nothing else, Gin defaults to debug mode automatically
}
