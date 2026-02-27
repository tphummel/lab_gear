package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/handlers"
	"github.com/tphummel/lab_gear/internal/middleware"
)

func main() {
	token := os.Getenv("API_TOKEN")
	if token == "" {
		log.Fatal("API_TOKEN environment variable is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./lab-assets.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	h := &handlers.Handler{DB: database}

	mux := http.NewServeMux()

	// Health check — no auth
	mux.HandleFunc("GET /healthz", h.Health)

	// Machine CRUD — Bearer token auth required
	mux.Handle("POST /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.CreateMachine)))
	mux.Handle("GET /api/v1/machines", middleware.Auth(token, http.HandlerFunc(h.ListMachines)))
	mux.Handle("GET /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.GetMachine)))
	mux.Handle("PUT /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.UpdateMachine)))
	mux.Handle("DELETE /api/v1/machines/{id}", middleware.Auth(token, http.HandlerFunc(h.DeleteMachine)))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
