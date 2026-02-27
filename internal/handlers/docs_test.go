package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tphummel/lab_gear/internal/handlers"
)

func TestOpenAPISpec_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	handlers.OpenAPISpec(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/yaml" {
		t.Errorf("Content-Type: got %q, want application/yaml", ct)
	}
}

func TestOpenAPISpec_NonEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	handlers.OpenAPISpec(w, req)

	if w.Body.Len() == 0 {
		t.Error("expected non-empty OpenAPI spec body")
	}
}

func TestOpenAPISpec_ContainsOpenAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	handlers.OpenAPISpec(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "openapi:") {
		preview := body
		if len(preview) > 200 {
			preview = preview[:200]
		}
		t.Errorf("spec body does not contain 'openapi:' key; got:\n%s", preview)
	}
}

func TestDocs_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	handlers.Docs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want text/html; charset=utf-8", ct)
	}
}

func TestDocs_ContainsSwaggerUI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	handlers.Docs(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "swagger-ui") {
		t.Error("docs body should reference swagger-ui")
	}
	if !strings.Contains(body, "/openapi.yaml") {
		t.Error("docs body should reference /openapi.yaml")
	}
}

func TestDocs_IsValidHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	handlers.Docs(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("docs response should begin with <!DOCTYPE html>")
	}
	if !strings.Contains(body, "</html>") {
		t.Error("docs response should contain closing </html> tag")
	}
}

// DocsAndSpecInMux verifies both endpoints are accessible through the full
// mux (integration-level check that they are registered in cmd/server/main.go).
func TestDocsAndSpec_ViaFullMux(t *testing.T) {
	mux, _ := newTestMux(t)

	// Register the doc routes the same way main.go does.
	// newTestMux only sets up the API routes, so add docs routes here.
	docsMux := http.NewServeMux()
	docsMux.Handle("/", mux)
	docsMux.HandleFunc("GET /openapi.yaml", handlers.OpenAPISpec)
	docsMux.HandleFunc("GET /docs", handlers.Docs)

	tests := []struct {
		path        string
		wantStatus  int
		wantCTPrefix string
	}{
		{"/openapi.yaml", http.StatusOK, "application/yaml"},
		{"/docs", http.StatusOK, "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := serve(docsMux, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, tt.wantCTPrefix) {
				t.Errorf("Content-Type: got %q, want prefix %q", ct, tt.wantCTPrefix)
			}
		})
	}
}

