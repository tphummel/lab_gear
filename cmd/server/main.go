package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/handlers"
	"github.com/tphummel/lab_gear/internal/middleware"
)

// loadConfig reads service configuration from environment variables and
// applies defaults. It returns an error when a required variable is absent.
func loadConfig() (token, dbPath, port string, err error) {
	token = os.Getenv("API_TOKEN")
	if token == "" {
		err = fmt.Errorf("API_TOKEN environment variable is required")
		return
	}
	dbPath = os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./lab_gear.db"
	}
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return
}

func main() {
	token, dbPath, port, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	h := &handlers.Handler{DB: database}

	mux := http.NewServeMux()

	// Health check — no auth
	mux.HandleFunc("GET /healthz", h.Health)

	// API docs — no auth
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPISpec)
	mux.HandleFunc("GET /docs", handlers.Docs)

	// Machine CRUD — Bearer token auth required
	mux.Handle("POST /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.CreateMachine)))
	mux.Handle("GET /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.ListMachines)))
	mux.Handle("GET /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.GetMachine)))
	mux.Handle("PUT /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.UpdateMachine)))
	mux.Handle("DELETE /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.DeleteMachine)))

	skipHealthz := func(r *http.Request) bool { return r.URL.Path == "/healthz" }
	handler := middleware.RequestLogger(slog.Default(), skipHealthz, mux)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
