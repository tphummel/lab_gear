package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tphummel/lab_gear/internal/middleware"
)

const testToken = "super-secret-token"

// okHandler is a trivial next handler that records it was reached.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestAuth(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantReach  bool // whether the next handler should be called
	}{
		{
			name:       "no header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantReach:  false,
		},
		{
			name:       "basic auth scheme",
			authHeader: "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
			wantReach:  false,
		},
		{
			name:       "bearer prefix only",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
			wantReach:  false,
		},
		{
			name:       "wrong token",
			authHeader: "Bearer wrong-token",
			wantStatus: http.StatusUnauthorized,
			wantReach:  false,
		},
		{
			name:       "correct token",
			authHeader: "Bearer " + testToken,
			wantStatus: http.StatusOK,
			wantReach:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.Auth(testToken, next)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if reached != tt.wantReach {
				t.Errorf("handler reached: got %v, want %v", reached, tt.wantReach)
			}
		})
	}
}

func TestAuth_CaseSensitive(t *testing.T) {
	// "bearer" (lowercase) must not be accepted — only "Bearer".
	handler := middleware.Auth(testToken, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer "+testToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("lowercase 'bearer': got %d, want 401", rec.Code)
	}
}

func TestAuth_TokenWithLeadingSpace(t *testing.T) {
	// A space before the token value should not authenticate.
	handler := middleware.Auth(testToken, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer  "+testToken) // two spaces
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token with leading space: got %d, want 401", rec.Code)
	}
}

func TestAuth_UnauthorizedResponseIsJSON(t *testing.T) {
	handler := middleware.Auth(testToken, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type on 401: got %q, want application/json", ct)
	}
}

func TestAuth_DifferentTokens(t *testing.T) {
	// Verifies that the middleware uses the token it was constructed with,
	// not some global state.
	const tokenA = "token-a"
	const tokenB = "token-b"

	handlerA := middleware.Auth(tokenA, okHandler)
	handlerB := middleware.Auth(tokenB, okHandler)

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.Header.Set("Authorization", "Bearer "+tokenA)

	recAonA := httptest.NewRecorder()
	handlerA.ServeHTTP(recAonA, reqA)
	if recAonA.Code != http.StatusOK {
		t.Errorf("tokenA on handlerA: got %d, want 200", recAonA.Code)
	}

	recAonB := httptest.NewRecorder()
	handlerB.ServeHTTP(recAonB, reqA)
	if recAonB.Code != http.StatusUnauthorized {
		t.Errorf("tokenA on handlerB: got %d, want 401", recAonB.Code)
	}
}
