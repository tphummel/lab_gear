package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const unauthorizedBody = `{"error":"unauthorized"}` + "\n"

// Auth returns a handler that requires a valid Bearer token before
// delegating to next. Responds with 401 if the header is missing or wrong.
// Token comparison uses constant-time equality to prevent timing attacks.
func Auth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		got := strings.TrimPrefix(authHeader, "Bearer ")
		if !strings.HasPrefix(authHeader, "Bearer ") || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(unauthorizedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}
