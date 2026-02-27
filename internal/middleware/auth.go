package middleware

import (
	"net/http"
	"strings"
)

const unauthorizedBody = `{"error":"unauthorized"}` + "\n"

// Auth returns a handler that requires a valid Bearer token before
// delegating to next. Responds with 401 if the header is missing or wrong.
func Auth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(unauthorizedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}
