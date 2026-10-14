package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eventide-manafield/manafield-noctemblith/internal/bridge"
)

func main() {
	noctemblith := strings.TrimSpace(os.Getenv("MANAFIELD_NOCTEMBLITH_HEALTH_URL"))
	if noctemblith == "" {
		noctemblith = "http://noctemblith-dev:3000/healthz/"
	}
	accountCore := strings.TrimSpace(os.Getenv("MANAFIELD_NOCTEMBLITH_ACCOUNT_HEALTH_URL"))
	if accountCore == "" {
		accountCore = "http://module-manafield-account-core:8080/manafield/health"
	}
	handler, err := bridge.New(noctemblith, accountCore, nil)
	if err != nil {
		slog.Error("invalid Noctemblith dependency configuration", "error", err)
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
	slog.Info("Noctemblith module adapter started", "port", port)
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Noctemblith module stopped", "error", err)
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
