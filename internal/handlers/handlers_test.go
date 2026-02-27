package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tphummel/lab_gear/internal/db"
	"github.com/tphummel/lab_gear/internal/handlers"
	"github.com/tphummel/lab_gear/internal/middleware"
	"github.com/tphummel/lab_gear/internal/models"
)

const apiToken = "test-token"

// newTestMux builds the same mux as main.go, backed by an in-memory DB.
// It returns both the mux (for serving requests) and the DB (for pre-seeding).
func newTestMux(t *testing.T) (http.Handler, *db.DB) {
	t.Helper()
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	h := &handlers.Handler{DB: d}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Health)
	mux.Handle("POST /api/v1/machines", middleware.Auth(apiToken, http.HandlerFunc(h.CreateMachine)))
	mux.Handle("GET /api/v1/machines", middleware.Auth(apiToken, http.HandlerFunc(h.ListMachines)))
	mux.Handle("GET /api/v1/machines/{id}", middleware.Auth(apiToken, http.HandlerFunc(h.GetMachine)))
	mux.Handle("PUT /api/v1/machines/{id}", middleware.Auth(apiToken, http.HandlerFunc(h.UpdateMachine)))
	mux.Handle("DELETE /api/v1/machines/{id}", middleware.Auth(apiToken, http.HandlerFunc(h.DeleteMachine)))

	return mux, d
}

// authReq builds a request with the test Bearer token already attached.
func authReq(method, path string, body []byte) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Authorization", "Bearer "+apiToken)
	return r
}

// serve is a small helper that runs a request through the mux and returns the recorder.
func serve(mux http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

// decodeBody unmarshals a recorder's body into v.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, w.Body.String())
	}
}

// --- Health ---

func TestHealth(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var body map[string]string
	decodeBody(t, w, &body)
	if body["status"] != "ok" {
		t.Errorf("status field: got %q, want %q", body["status"], "ok")
	}
}

// Health must not require auth.
func TestHealth_NoAuth(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Errorf("healthz without auth: got %d, want 200", w.Code)
	}
}

// --- Auth guard on protected routes ---

func TestProtectedRoutes_RequireAuth(t *testing.T) {
	mux, _ := newTestMux(t)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/machines"},
		{http.MethodGet, "/api/v1/machines"},
		{http.MethodGet, "/api/v1/machines/some-id"},
		{http.MethodPut, "/api/v1/machines/some-id"},
		{http.MethodDelete, "/api/v1/machines/some-id"},
	}

	for _, rt := range routes {
		t.Run(fmt.Sprintf("%s %s", rt.method, rt.path), func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			// deliberately no Authorization header
			w := serve(mux, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 without auth, got %d", w.Code)
			}
		})
	}
}

// --- CreateMachine ---

func TestCreateMachine_Valid(t *testing.T) {
	mux, _ := newTestMux(t)

	payload := map[string]any{
		"name":       "pve2",
		"kind":       "proxmox",
		"make":       "Dell",
		"model":      "OptiPlex 7050",
		"cpu":        "i7-7700",
		"ram_gb":     32,
		"storage_tb": 1.0,
		"location":   "office rack",
	}
	body, _ := json.Marshal(payload)
	w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201\nbody: %s", w.Code, w.Body.String())
	}

	var m models.Machine
	decodeBody(t, w, &m)

	if m.ID == "" {
		t.Error("ID should be non-empty")
	}
	if m.Name != "pve2" {
		t.Errorf("Name: got %q, want %q", m.Name, "pve2")
	}
	if m.Kind != "proxmox" {
		t.Errorf("Kind: got %q, want %q", m.Kind, "proxmox")
	}
	if m.RAMGB != 32 {
		t.Errorf("RAMGB: got %d, want 32", m.RAMGB)
	}
	if m.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if m.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestCreateMachine_ValidationErrors(t *testing.T) {
	mux, _ := newTestMux(t)

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "missing name",
			payload: map[string]any{"kind": "proxmox", "make": "Dell", "model": "X"},
		},
		{
			name:    "missing kind",
			payload: map[string]any{"name": "pve2", "make": "Dell", "model": "X"},
		},
		{
			name:    "missing make",
			payload: map[string]any{"name": "pve2", "kind": "proxmox", "model": "X"},
		},
		{
			name:    "missing model",
			payload: map[string]any{"name": "pve2", "kind": "proxmox", "make": "Dell"},
		},
		{
			name:    "invalid kind",
			payload: map[string]any{"name": "pve2", "kind": "mainframe", "make": "IBM", "model": "Z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
			if w.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400\nbody: %s", w.Code, w.Body.String())
			}
			var resp map[string]string
			decodeBody(t, w, &resp)
			if resp["error"] == "" {
				t.Error("expected non-empty error field")
			}
		})
	}
}

func TestCreateMachine_InvalidJSON(t *testing.T) {
	mux, _ := newTestMux(t)
	req := authReq(http.MethodPost, "/api/v1/machines", []byte("not-json"))
	w := serve(mux, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestCreateMachine_AllKinds(t *testing.T) {
	mux, _ := newTestMux(t)

	for kind := range models.ValidKinds {
		t.Run(kind, func(t *testing.T) {
			payload := map[string]any{
				"name":  "test",
				"kind":  kind,
				"make":  "Acme",
				"model": "Model-1",
			}
			body, _ := json.Marshal(payload)
			w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
			if w.Code != http.StatusCreated {
				t.Errorf("kind %q: status got %d, want 201\nbody: %s", kind, w.Code, w.Body.String())
			}
		})
	}
}

// --- ListMachines ---

func TestListMachines_Empty(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var machines []models.Machine
	decodeBody(t, w, &machines)
	if len(machines) != 0 {
		t.Errorf("expected empty array, got %d items", len(machines))
	}
}

func TestListMachines_ReturnsAll(t *testing.T) {
	mux, _ := newTestMux(t)

	for i := range 3 {
		payload := map[string]any{
			"name":  fmt.Sprintf("node%d", i),
			"kind":  "proxmox",
			"make":  "Dell",
			"model": "R640",
		}
		body, _ := json.Marshal(payload)
		w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
		if w.Code != http.StatusCreated {
			t.Fatalf("create machine %d: %d %s", i, w.Code, w.Body.String())
		}
	}

	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines", nil))
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var machines []models.Machine
	decodeBody(t, w, &machines)
	if len(machines) != 3 {
		t.Errorf("expected 3 machines, got %d", len(machines))
	}
}

func TestListMachines_KindFilter(t *testing.T) {
	mux, _ := newTestMux(t)

	creates := []struct {
		name string
		kind string
	}{
		{"pve1", "proxmox"},
		{"pve2", "proxmox"},
		{"nas01", "nas"},
		{"pi01", "sbc"},
	}
	for _, c := range creates {
		payload := map[string]any{"name": c.name, "kind": c.kind, "make": "X", "model": "Y"}
		body, _ := json.Marshal(payload)
		if w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body)); w.Code != http.StatusCreated {
			t.Fatalf("create %q: %s", c.name, w.Body.String())
		}
	}

	tests := []struct {
		kind string
		want int
	}{
		{"proxmox", 2},
		{"nas", 1},
		{"sbc", 1},
		{"laptop", 0},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			w := serve(mux, authReq(http.MethodGet, "/api/v1/machines?kind="+tt.kind, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", w.Code)
			}
			var machines []models.Machine
			decodeBody(t, w, &machines)
			if len(machines) != tt.want {
				t.Errorf("kind=%q: got %d machines, want %d", tt.kind, len(machines), tt.want)
			}
		})
	}
}

func TestListMachines_InvalidKindFilter(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines?kind=mainframe", nil))
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

// --- GetMachine ---

func TestGetMachine_Found(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create a machine first.
	payload := map[string]any{"name": "pi01", "kind": "sbc", "make": "Raspberry Pi", "model": "4B"}
	body, _ := json.Marshal(payload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	// Fetch it.
	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines/"+created.ID, nil))
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200\nbody: %s", w.Code, w.Body.String())
	}

	var got models.Machine
	decodeBody(t, w, &got)
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}
	if got.Name != "pi01" {
		t.Errorf("Name: got %q, want %q", got.Name, "pi01")
	}
}

func TestGetMachine_NotFound(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines/does-not-exist", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
	var body map[string]string
	decodeBody(t, w, &body)
	if body["error"] == "" {
		t.Error("expected non-empty error field")
	}
}

// --- UpdateMachine ---

func TestUpdateMachine_Valid(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create.
	createPayload := map[string]any{"name": "nas01", "kind": "nas", "make": "Synology", "model": "DS920+"}
	body, _ := json.Marshal(createPayload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	// Update.
	updatePayload := map[string]any{
		"name":       "nas01",
		"kind":       "nas",
		"make":       "Synology",
		"model":      "DS923+",
		"ram_gb":     8,
		"storage_tb": 40.0,
		"notes":      "upgraded disks",
	}
	body, _ = json.Marshal(updatePayload)
	w := serve(mux, authReq(http.MethodPut, "/api/v1/machines/"+created.ID, body))
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200\nbody: %s", w.Code, w.Body.String())
	}

	var updated models.Machine
	decodeBody(t, w, &updated)

	if updated.Model != "DS923+" {
		t.Errorf("Model: got %q, want %q", updated.Model, "DS923+")
	}
	if updated.RAMGB != 8 {
		t.Errorf("RAMGB: got %d, want 8", updated.RAMGB)
	}
	if updated.StorageTB != 40.0 {
		t.Errorf("StorageTB: got %f, want 40.0", updated.StorageTB)
	}
	if updated.Notes != "upgraded disks" {
		t.Errorf("Notes: got %q, want %q", updated.Notes, "upgraded disks")
	}
	// ID and CreatedAt must be preserved.
	if updated.ID != created.ID {
		t.Errorf("ID changed: got %q, want %q", updated.ID, created.ID)
	}
	// The DB stores timestamps as RFC3339 (second precision). Truncate both
	// sides before comparing so the test is robust to sub-second differences
	// between the in-memory create response and the post-DB-round-trip value.
	if !updated.CreatedAt.Truncate(time.Second).Equal(created.CreatedAt.Truncate(time.Second)) {
		t.Errorf("CreatedAt changed: got %v, want %v", updated.CreatedAt, created.CreatedAt)
	}
}

func TestUpdateMachine_NotFound(t *testing.T) {
	mux, _ := newTestMux(t)
	payload := map[string]any{"name": "x", "kind": "laptop", "make": "Apple", "model": "M3"}
	body, _ := json.Marshal(payload)
	w := serve(mux, authReq(http.MethodPut, "/api/v1/machines/no-such-id", body))
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestUpdateMachine_ValidationErrors(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create a machine to update against.
	createPayload := map[string]any{"name": "ws01", "kind": "workstation", "make": "System76", "model": "Thelio"}
	body, _ := json.Marshal(createPayload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "missing name",
			payload: map[string]any{"kind": "workstation", "make": "System76", "model": "Thelio"},
		},
		{
			name:    "invalid kind",
			payload: map[string]any{"name": "ws01", "kind": "toaster", "make": "System76", "model": "Thelio"},
		},
		{
			name:    "empty body fields",
			payload: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			w := serve(mux, authReq(http.MethodPut, "/api/v1/machines/"+created.ID, body))
			if w.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400\nbody: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateMachine_InvalidJSON(t *testing.T) {
	mux, _ := newTestMux(t)

	createPayload := map[string]any{"name": "ws01", "kind": "workstation", "make": "Dell", "model": "Precision"}
	body, _ := json.Marshal(createPayload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	req := authReq(http.MethodPut, "/api/v1/machines/"+created.ID, []byte("bad json"))
	w := serve(mux, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

// --- DeleteMachine ---

func TestDeleteMachine_Found(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create.
	payload := map[string]any{"name": "old-box", "kind": "bare_metal", "make": "HP", "model": "DL380"}
	body, _ := json.Marshal(payload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	// Delete.
	w := serve(mux, authReq(http.MethodDelete, "/api/v1/machines/"+created.ID, nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204\nbody: %s", w.Code, w.Body.String())
	}

	// Confirm it's gone.
	getW := serve(mux, authReq(http.MethodGet, "/api/v1/machines/"+created.ID, nil))
	if getW.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getW.Code)
	}
}

func TestDeleteMachine_NotFound(t *testing.T) {
	mux, _ := newTestMux(t)
	w := serve(mux, authReq(http.MethodDelete, "/api/v1/machines/ghost-id", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

// --- Content-Type ---

func TestResponseContentType(t *testing.T) {
	mux, _ := newTestMux(t)

	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines", nil))
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestErrorResponseContentType(t *testing.T) {
	mux, _ := newTestMux(t)

	// A 400 error (validation failure) should also return application/json.
	payload := map[string]any{"kind": "proxmox"} // missing required fields
	body, _ := json.Marshal(payload)
	w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type on error: got %q, want application/json", ct)
	}
}

// --- Edge cases ---

func TestCreateMachine_EmptyBody(t *testing.T) {
	mux, _ := newTestMux(t)
	// An empty body cannot be decoded as JSON — expect 400.
	req := authReq(http.MethodPost, "/api/v1/machines", []byte{})
	req.Header.Set("Content-Type", "application/json")
	w := serve(mux, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty body: got %d, want 400", w.Code)
	}
}

func TestUpdateMachine_EmptyBody(t *testing.T) {
	mux, _ := newTestMux(t)

	// Create a machine to target.
	createPayload := map[string]any{"name": "ws", "kind": "workstation", "make": "Dell", "model": "XPS"}
	body, _ := json.Marshal(createPayload)
	createW := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: %s", createW.Body.String())
	}
	var created models.Machine
	decodeBody(t, createW, &created)

	req := authReq(http.MethodPut, "/api/v1/machines/"+created.ID, []byte{})
	req.Header.Set("Content-Type", "application/json")
	w := serve(mux, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty body: got %d, want 400", w.Code)
	}
}

func TestCreateMachine_UTF8Fields(t *testing.T) {
	mux, _ := newTestMux(t)

	payload := map[string]any{
		"name":     "节点1",                         // Chinese characters
		"kind":     "sbc",
		"make":     "Raspberry Pî",                // Unicode in make
		"model":    "Modèle-Spécial",              // French accents in model
		"location": "Büro Regal 3",               // German umlaut
		"notes":    "正常运行 ✓",                    // Mixed script + emoji
	}
	body, _ := json.Marshal(payload)
	w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body))
	if w.Code != http.StatusCreated {
		t.Errorf("UTF-8 fields: got %d, want 201\nbody: %s", w.Code, w.Body.String())
	}

	var m models.Machine
	decodeBody(t, w, &m)
	if m.Name != "节点1" {
		t.Errorf("Name round-trip: got %q, want %q", m.Name, "节点1")
	}
	if m.Notes != "正常运行 ✓" {
		t.Errorf("Notes round-trip: got %q", m.Notes)
	}
}

func TestListMachines_UTF8RoundTrip(t *testing.T) {
	mux, _ := newTestMux(t)

	payload := map[string]any{"name": "пи01", "kind": "sbc", "make": "RPi", "model": "4B"}
	body, _ := json.Marshal(payload)
	if w := serve(mux, authReq(http.MethodPost, "/api/v1/machines", body)); w.Code != http.StatusCreated {
		t.Fatalf("create: %s", w.Body.String())
	}

	w := serve(mux, authReq(http.MethodGet, "/api/v1/machines", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var machines []models.Machine
	decodeBody(t, w, &machines)
	if len(machines) != 1 || machines[0].Name != "пи01" {
		t.Errorf("UTF-8 name not preserved in list: %+v", machines)
	}
}
