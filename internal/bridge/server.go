// Package bridge implements the Manafield Module Protocol health contract
// for an independently deployed Noctite service. It never proxies user traffic.
package bridge

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const moduleVersion = "0.0.1"

type Handler struct {
	target string
	client *http.Client
}

func New(target string, client *http.Client) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Path != "/healthz/" ||
		u.Opaque != "" || u.RawPath != "" || u.Hostname() == "" {
		return nil, errors.New("Noctite health endpoint must be an internal HTTP /healthz/ URL")
	}
	// Only a configured service hostname is accepted, never a user-selected URL.
	// In an actual deployment, that service must be connected to the adapter
	// through the private module network and must not expose database credentials.
	if strings.TrimSpace(u.Hostname()) != u.Hostname() {
		return nil, errors.New("invalid internal service host")
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}
	h := &Handler{target: u.String(), client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /manafield/health", h.health)
	mux.HandleFunc("GET /manafield/noctite/status", h.health)
	return mux, nil
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.target, nil)
	ready := false
	if err == nil {
		res, reqErr := h.client.Do(req)
		if reqErr == nil {
			ready = res.StatusCode == http.StatusOK
			_ = res.Body.Close()
		}
	}
	status := http.StatusOK
	state := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "degraded"
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       state,
		"service":      "manafield-noctite",
		"version":      moduleVersion,
		"noctiteReady": ready,
	})
}
