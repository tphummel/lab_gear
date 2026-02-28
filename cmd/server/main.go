package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/handlers"
	"github.com/tphummel/lab_gear/internal/middleware"
)

// version and commit are injected at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
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

	h := &handlers.Handler{DB: database, Version: version, Commit: commit}

	mux := http.NewServeMux()

	// Health check — no auth
	mux.HandleFunc("GET /healthz", h.Health)

	// Prometheus metrics — no auth
	mux.Handle("GET /metrics", promhttp.Handler())

	// API docs — no auth
	mux.HandleFunc("GET /openapi.yaml", handlers.OpenAPISpec)
	mux.HandleFunc("GET /docs", handlers.Docs)

	// Machine CRUD — Bearer token auth required
	mux.Handle("POST /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.CreateMachine)))
	mux.Handle("GET /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.ListMachines)))
	mux.Handle("GET /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.GetMachine)))
	mux.Handle("PUT /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.UpdateMachine)))
	mux.Handle("DELETE /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.DeleteMachine)))

	skip := func(r *http.Request) bool {
		return r.URL.Path == "/healthz" || r.URL.Path == "/metrics"
	}
	handler := middleware.RequestLogger(slog.Default(), skip, mux)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	if err := database.Close(); err != nil {
		log.Printf("database close error: %v", err)
	}
	log.Println("server stopped")
}
