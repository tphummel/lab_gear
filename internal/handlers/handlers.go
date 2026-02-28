package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/models"
)

// Handler holds shared dependencies for HTTP handlers.
type Handler struct {
	DB      *db.DB
	Version string
	Commit  string
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Health handles GET /healthz — no auth required.
// Returns 503 if the database is unreachable.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": h.Version,
		"commit":  h.Commit,
	})
}

// CreateMachine handles POST /api/v1/machines.
func (h *Handler) CreateMachine(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req models.Machine
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" || req.Kind == "" || req.Make == "" || req.Model == "" {
		writeError(w, http.StatusBadRequest, "name, kind, make, and model are required")
		return
	}
	if !models.ValidKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "invalid kind")
		return
	}

	now := time.Now().UTC()
	req.ID = uuid.New().String()
	req.CreatedAt = now
	req.UpdatedAt = now

	if err := h.DB.Create(&req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create machine")
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

// ListMachines handles GET /api/v1/machines with an optional ?kind= filter.
func (h *Handler) ListMachines(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "" && !models.ValidKinds[kind] {
		writeError(w, http.StatusBadRequest, "invalid kind")
		return
	}

	machines, err := h.DB.List(kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list machines")
		return
	}

	if machines == nil {
		machines = []*models.Machine{}
	}
	writeJSON(w, http.StatusOK, machines)
}

// GetMachine handles GET /api/v1/machines/{id}.
func (h *Handler) GetMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	machine, err := h.DB.GetByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get machine")
		return
	}
	writeJSON(w, http.StatusOK, machine)
}

// UpdateMachine handles PUT /api/v1/machines/{id}.
func (h *Handler) UpdateMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := h.DB.GetByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get machine")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req models.Machine
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" || req.Kind == "" || req.Make == "" || req.Model == "" {
		writeError(w, http.StatusBadRequest, "name, kind, make, and model are required")
		return
	}
	if !models.ValidKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "invalid kind")
		return
	}

	req.ID = id
	req.CreatedAt = existing.CreatedAt
	req.UpdatedAt = time.Now().UTC()

	if err := h.DB.Update(&req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update machine")
		return
	}

	writeJSON(w, http.StatusOK, req)
}

// DeleteMachine handles DELETE /api/v1/machines/{id}.
func (h *Handler) DeleteMachine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.DB.Delete(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "machine not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete machine")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
