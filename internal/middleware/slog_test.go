package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tphummel/lab_gear/internal/middleware"
)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestRequestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	handler := middleware.RequestLogger(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/machines", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	for _, key := range []string{"method", "path", "status", "duration", "remote_addr"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("log entry missing key %q", key)
		}
	}
	if entry["method"] != http.MethodGet {
		t.Errorf("method: got %v, want %v", entry["method"], http.MethodGet)
	}
	if entry["path"] != "/api/v1/machines" {
		t.Errorf("path: got %v, want %v", entry["path"], "/api/v1/machines")
	}
	if int(entry["status"].(float64)) != http.StatusOK {
		t.Errorf("status: got %v, want %d", entry["status"], http.StatusOK)
	}
}

func TestRequestLogger_SkipsHealthcheck(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	skip := func(r *http.Request) bool { return r.URL.Path == "/healthz" }
	handler := middleware.RequestLogger(logger, skip, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if buf.Len() > 0 {
		t.Errorf("expected no log output for healthcheck, got: %s", buf.String())
	}
}

func TestRequestLogger_LogsNonSkippedPaths(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	skip := func(r *http.Request) bool { return r.URL.Path == "/healthz" }
	handler := middleware.RequestLogger(logger, skip, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/machines", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if buf.Len() == 0 {
		t.Error("expected log output for non-skipped path, got none")
	}
}

func TestRequestLogger_CapturesNonOKStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	handler := middleware.RequestLogger(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if int(entry["status"].(float64)) != http.StatusNotFound {
		t.Errorf("status: got %v, want %d", entry["status"], http.StatusNotFound)
	}
}

func TestRequestLogger_DefaultsTo200(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	// Handler writes body without calling WriteHeader; status should default to 200.
	handler := middleware.RequestLogger(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello")) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if int(entry["status"].(float64)) != http.StatusOK {
		t.Errorf("status: got %v, want 200", entry["status"])
	}
}

func TestRequestLogger_NilSkip(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	// nil skip function should log all requests, including healthcheck path.
	handler := middleware.RequestLogger(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if buf.Len() == 0 {
		t.Error("expected log output when skip is nil, got none")
	}
}
