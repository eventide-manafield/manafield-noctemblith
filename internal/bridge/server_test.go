package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func dependencies(t *testing.T, nStatus, aStatus int, aBody string) (string, string, func()) {
	t.Helper()
	n := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/healthz/" {
			t.Errorf("unexpected Noctemblith request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(nStatus)
	}))
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/manafield/health" {
			t.Errorf("unexpected Account request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(aStatus)
		_, _ = w.Write([]byte(aBody))
	}))
	return n.URL + "/healthz/", a.URL + "/manafield/health", func() { n.Close(); a.Close() }
}

const goodAccount = `{"status":"ok","service":"manafield-account-core","databaseStatus":"ready"}`

func TestHealthyDependencies(t *testing.T) {
	n, a, cleanup := dependencies(t, 200, 200, goodAccount)
	defer cleanup()
	h, err := New(n, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/manafield/health", "/manafield/noctemblith/status"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s status=%d: %s", path, w.Code, w.Body.String())
		}
		var v struct {
			Status               string
			Service              string
			NoctemblithReady     bool
			AccountCoreReady     bool
			AccountDatabaseReady bool
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.Status != "ok" || v.Service != "manafield-noctemblith" || !v.NoctemblithReady || !v.AccountCoreReady || !v.AccountDatabaseReady {
			t.Fatalf("wrong readiness: %+v", v)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("health response must not be cached")
		}
	}
}

func TestUnavailableDependenciesFailClosed(t *testing.T) {
	cases := []struct {
		name                 string
		nStatus, aStatus     int
		aBody                string
		wantN, wantA, wantDB bool
	}{
		{"Noctemblith unavailable", 503, 200, goodAccount, false, true, true},
		{"Account unavailable", 200, 503, goodAccount, true, false, false},
		{"Account database waiting", 200, 200, `{"status":"degraded","service":"manafield-account-core","databaseStatus":"waiting"}`, true, false, false},
		{"Account missing schema", 200, 200, `{"status":"ok","service":"manafield-account-core"}`, true, false, false},
		{"Account wrong service", 200, 200, `{"status":"ok","service":"evil","databaseStatus":"ready"}`, true, false, true},
		{"Account malformed JSON", 200, 200, `not json`, true, false, false},
		{"Noctemblith redirect", 302, 200, goodAccount, false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, a, cleanup := dependencies(t, tc.nStatus, tc.aStatus, tc.aBody)
			defer cleanup()
			h, err := New(n, a, nil)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/manafield/health", nil))
			if w.Code != 503 {
				t.Fatalf("must degrade: %d %s", w.Code, w.Body.String())
			}
			var v struct {
				NoctemblithReady     bool
				AccountCoreReady     bool
				AccountDatabaseReady bool
			}
			if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if v.NoctemblithReady != tc.wantN || v.AccountCoreReady != tc.wantA || v.AccountDatabaseReady != tc.wantDB {
				t.Fatalf("unexpected readiness: %+v", v)
			}
		})
	}
}

func TestNetworkFailureFailsClosed(t *testing.T) {
	n, a, cleanup := dependencies(t, 200, 200, goodAccount)
	cleanup()
	client := &http.Client{Timeout: 100 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	h, err := New(n, a, client)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/manafield/health", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"accountCoreReady":false`) {
		t.Fatalf("network failure: %d %s", w.Code, w.Body.String())
	}
}

func TestRejectInvalidDependencyURLs(t *testing.T) {
	for _, bad := range []string{
		"", "/healthz/", "https://host/healthz/", "http://host/other",
		"http://user:pass@host/healthz/", "http://host/healthz/?secret=a",
		"http://host/healthz/#fragment", "http://host/healthz/%2f",
	} {
		if _, err := New(bad, "http://account:8080/manafield/health", nil); err == nil {
			t.Errorf("accepted invalid Noctemblith endpoint %q", bad)
		}
	}
	for _, bad := range []string{
		"", "https://account/manafield/health", "http://account:8080/",
		"http://account:8080/manafield/health?token=x",
		"http://user:pass@account/manafield/health",
	} {
		if _, err := New("http://noctemblith-dev:3000/healthz/", bad, nil); err == nil {
			t.Errorf("accepted invalid Account endpoint %q", bad)
		}
	}
}

func TestNoWriteEndpoint(t *testing.T) {
	h, err := New("http://noctemblith-dev:3000/healthz/", "http://account:8080/manafield/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/manafield/noctemblith/status", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("writes must be refused: %d", w.Code)
	}
}
