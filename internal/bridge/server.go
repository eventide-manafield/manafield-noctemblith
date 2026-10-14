// Package bridge implements a read-only Manafield Module Protocol status adapter
// for an independently deployed Noctemblith service and Manafield Account Core.
// Both applications own their own database state. This adapter has no DB access,
// OAuth tokens, browser sessions or public ingress routes.
package bridge

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const moduleVersion = "0.1.0"

type Handler struct {
	noctemblithURL string
	accountURL     string
	client         *http.Client
}

func internalHealthURL(value, path string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Path != path ||
		u.RawPath != "" || u.Opaque != "" ||
		strings.TrimSpace(u.Hostname()) != u.Hostname() {
		return "", errors.New("dependency URL must be a configured internal HTTP health endpoint")
	}
	return u.String(), nil
}

func New(noctemblith, accountCore string, client *http.Client) (http.Handler, error) {
	nurl, err := internalHealthURL(noctemblith, "/healthz/")
	if err != nil {
		return nil, err
	}
	aurl, err := internalHealthURL(accountCore, "/manafield/health")
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	handler := &Handler{noctemblithURL: nurl, accountURL: aurl, client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /manafield/health", handler.status)
	mux.HandleFunc("GET /manafield/noctemblith/status", handler.status)
	return mux, nil
}

func (h *Handler) check(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return h.client.Do(req)
}

func (h *Handler) noctemblithReady() bool {
	response, err := h.check(h.noctemblithURL)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func (h *Handler) accountStatus() (bool, bool) {
	response, err := h.check(h.accountURL)
	if err != nil {
		return false, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, false
	}
	var body struct {
		Status         string `json:"status"`
		Service        string `json:"service"`
		DatabaseStatus string `json:"databaseStatus"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&body); err != nil {
		return false, false
	}
	dbReady := body.DatabaseStatus == "ready"
	return body.Status == "ok" && body.Service == "manafield-account-core" && dbReady, dbReady
}

func (h *Handler) status(w http.ResponseWriter, _ *http.Request) {
	// The two checks are independent: one missing dependency must not hide
	// the readiness result of the other dependency.
	noctemblithReady := h.noctemblithReady()
	accountReady, accountDatabaseReady := h.accountStatus()
	ready := noctemblithReady && accountReady

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	code, status := http.StatusOK, "ok"
	if !ready {
		code, status = http.StatusServiceUnavailable, "degraded"
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":               status,
		"service":              "manafield-noctemblith",
		"version":              moduleVersion,
		"noctemblithReady":     noctemblithReady,
		"accountCoreReady":     accountReady,
		"accountDatabaseReady": accountDatabaseReady,
	})
}
