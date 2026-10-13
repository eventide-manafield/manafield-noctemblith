package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthyNoctiteMapsToManafieldHealth(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz/" || r.Method != http.MethodGet {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer remote.Close()
	mux, err := New(remote.URL+"/healthz/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/manafield/health", "/manafield/noctite/status"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, w.Code)
		}
		var data struct {
			Status       string
			NoctiteReady bool
		}
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Status != "ok" || !data.NoctiteReady {
			t.Fatalf("%s: unexpected result %s error %v", path, w.Body.String(), err)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("health cacheable")
		}
	}
}

func TestNoctiteUnhealthyFailsClosed(t *testing.T) {
	for _, remoteStatus := range []int{http.StatusServiceUnavailable, http.StatusNotFound, http.StatusFound} {
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(remoteStatus)
		}))
		mux, err := New(remote.URL+"/healthz/", nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/manafield/health", nil))
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "\"noctiteReady\":false") {
			t.Fatalf("upstream %d: %d %s", remoteStatus, w.Code, w.Body.String())
		}
		remote.Close()
	}
}

func TestNetworkFailureDoesNotPretendHealthy(t *testing.T) {
	client := &http.Client{Timeout: 100 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := remote.URL + "/healthz/"
	remote.Close()
	mux, err := New(url, client)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/manafield/health", nil))
	if w.Code != 503 {
		t.Fatalf("unavailable backend should return 503, got %d", w.Code)
	}
}

func TestRejectsUnsafeUpstreamTargets(t *testing.T) {
	for _, url := range []string{
		"", "/healthz/", "https://example.org/healthz/", "http://noctite:3000/",
		"http://user:password@noctite:3000/healthz/",
		"http://noctite:3000/healthz/?token=a",
		"http://noctite:3000/healthz/#secret",
		"http://noctite:3000/healthz/%2f",
	} {
		if _, err := New(url, nil); err == nil {
			t.Errorf("accepted invalid target %q", url)
		}
	}
}

func TestNoWriteEndpoints(t *testing.T) {
	mux, err := New("http://noctite:3000/healthz/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/manafield/noctite/status", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST unexpectedly allowed: %d", w.Code)
	}
}
