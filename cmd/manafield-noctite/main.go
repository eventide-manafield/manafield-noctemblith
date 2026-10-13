package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eventide-manafield/manafield-noctite/internal/bridge"
)

func main() {
	target := strings.TrimSpace(os.Getenv("MANAFIELD_NOCTITE_HEALTH_URL"))
	if target == "" {
		target = "http://noctite:3000/healthz/"
	}
	handler, err := bridge.New(target, nil)
	if err != nil {
		slog.Error("invalid Noctite upstream configuration", "error", err)
		os.Exit(1)
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	if !validPort(port) {
		slog.Error("invalid module listen port")
		os.Exit(1)
	}
	slog.Info("Noctite module adapter started", "port", port)
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Noctite module stopped", "error", err)
		os.Exit(1)
	}
}

func validPort(value string) bool {
	if value == "" || len(value) > 5 {
		return false
	}
	var n int
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 65535
}
