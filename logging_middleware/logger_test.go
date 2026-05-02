package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockServer returns a test server that always responds with the given status
// and a LogResponse body.
func mockServer(t *testing.T, status int, logID string, assertBody func(LogRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req LogRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("could not decode request body: %v", err)
		}
		if assertBody != nil {
			assertBody(req)
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(LogResponse{LogID: logID, Message: "log created successfully"})
	}))
}

func newTestLogger(srv *httptest.Server) *Logger {
	return &Logger{
		HTTPClient:  srv.Client(),
		endpointURL: srv.URL,
	}
}

// ── Happy path ────────────────────────────────────────────────────────────────

func TestLog_Success(t *testing.T) {
	srv := mockServer(t, http.StatusOK, "abc-123", func(req LogRequest) {
		if req.Stack != "backend" {
			t.Errorf("stack: want backend, got %s", req.Stack)
		}
		if req.Level != "info" {
			t.Errorf("level: want info, got %s", req.Level)
		}
		if req.Package != "handler" {
			t.Errorf("package: want handler, got %s", req.Package)
		}
		if req.Message != "unit test log entry" {
			t.Errorf("message: want 'unit test log entry', got %s", req.Message)
		}
	})
	defer srv.Close()

	l := newTestLogger(srv)
	logID, err := l.Log("backend", "info", "handler", "unit test log entry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logID != "abc-123" {
		t.Errorf("logID: want abc-123, got %s", logID)
	}
}

// ── Case normalisation ────────────────────────────────────────────────────────

func TestLog_CaseNormalisation(t *testing.T) {
	srv := mockServer(t, http.StatusOK, "ok", func(req LogRequest) {
		if req.Stack != "backend" {
			t.Errorf("expected normalised stack=backend, got %s", req.Stack)
		}
		if req.Level != "warn" {
			t.Errorf("expected normalised level=warn, got %s", req.Level)
		}
		if req.Package != "db" {
			t.Errorf("expected normalised package=db, got %s", req.Package)
		}
	})
	defer srv.Close()

	l := newTestLogger(srv)
	_, err := l.Log("BACKEND", "WARN", "DB", "case normalisation test")
	if err != nil {
		t.Errorf("case normalisation should not fail, got: %v", err)
	}
}

// ── Validation errors ─────────────────────────────────────────────────────────

func TestLog_InvalidStack(t *testing.T) {
	l := &Logger{HTTPClient: http.DefaultClient, endpointURL: "http://unused"}
	_, err := l.Log("mobile", "info", "handler", "test")
	if err == nil {
		t.Error("expected validation error for invalid stack")
	}
}

func TestLog_InvalidLevel(t *testing.T) {
	l := &Logger{HTTPClient: http.DefaultClient, endpointURL: "http://unused"}
	_, err := l.Log("backend", "verbose", "handler", "test")
	if err == nil {
		t.Error("expected validation error for invalid level")
	}
}

func TestLog_InvalidPackage(t *testing.T) {
	l := &Logger{HTTPClient: http.DefaultClient, endpointURL: "http://unused"}
	_, err := l.Log("backend", "info", "unknown_pkg", "test")
	if err == nil {
		t.Error("expected validation error for invalid package")
	}
}

func TestLog_EmptyMessage(t *testing.T) {
	l := &Logger{HTTPClient: http.DefaultClient, endpointURL: "http://unused"}
	_, err := l.Log("backend", "info", "handler", "")
	if err == nil {
		t.Error("expected error for empty message")
	}
}

// ── All valid log levels ──────────────────────────────────────────────────────

func TestLog_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "fatal"}
	for _, lvl := range levels {
		t.Run(lvl, func(t *testing.T) {
			srv := mockServer(t, http.StatusOK, "id-"+lvl, nil)
			defer srv.Close()
			l := newTestLogger(srv)
			_, err := l.Log("backend", lvl, "service", "testing level "+lvl)
			if err != nil {
				t.Errorf("level %s should be valid, got: %v", lvl, err)
			}
		})
	}
}

// ── All valid backend packages ────────────────────────────────────────────────

func TestLog_AllBackendPackages(t *testing.T) {
	pkgs := []string{
		"cache", "controller", "cron_job", "db", "domain",
		"handler", "repository", "route", "service",
		"auth", "config", "middleware", "utils",
	}
	for _, pkg := range pkgs {
		t.Run(pkg, func(t *testing.T) {
			srv := mockServer(t, http.StatusOK, "id", nil)
			defer srv.Close()
			l := newTestLogger(srv)
			_, err := l.Log("backend", "info", pkg, "testing package "+pkg)
			if err != nil {
				t.Errorf("package %s should be valid, got: %v", pkg, err)
			}
		})
	}
}

// ── Authorization header ──────────────────────────────────────────────────────

func TestLog_AuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-key" {
			t.Errorf("expected Authorization: Bearer secret-key, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LogResponse{LogID: "ok", Message: "log created successfully"})
	}))
	defer srv.Close()

	l := &Logger{
		HTTPClient:  srv.Client(),
		APIKey:      "secret-key",
		endpointURL: srv.URL,
	}
	_, err := l.Log("backend", "debug", "auth", "testing auth header injection")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
