package apiclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tphummel/lab_gear/terraform-provider-lab_gear/internal/apiclient"
)

const testToken = "test-api-key"

// newTestServer starts an httptest.Server that behaves like the lab_gear REST
// API. handler is called with the matched method and path; it writes the
// desired response.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *apiclient.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := apiclient.NewClient(srv.URL, testToken)
	return srv, client
}

// writeMachine serialises m as JSON with statusCode.
func writeMachine(w http.ResponseWriter, statusCode int, m apiclient.Machine) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(m)
}

// --- CreateMachine ---

func TestClient_CreateMachine_Success(t *testing.T) {
	want := apiclient.Machine{
		ID:    "uuid-1",
		Name:  "pve1",
		Kind:  "proxmox",
		Make:  "Dell",
		Model: "R640",
	}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/machines" {
			t.Errorf("path: got %q, want /api/v1/machines", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}
		writeMachine(w, http.StatusCreated, want)
	})

	got, err := client.CreateMachine(context.Background(), apiclient.Machine{
		Name:  "pve1",
		Kind:  "proxmox",
		Make:  "Dell",
		Model: "R640",
	})
	if err != nil {
		t.Fatalf("CreateMachine: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
}

func TestClient_CreateMachine_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	_, err := client.CreateMachine(context.Background(), apiclient.Machine{})
	if err == nil {
		t.Fatal("expected error on non-201 response, got nil")
	}
}

// --- GetMachine ---

func TestClient_GetMachine_Found(t *testing.T) {
	want := apiclient.Machine{ID: "uuid-2", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS920+"}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/machines/uuid-2" {
			t.Errorf("path: got %q, want /api/v1/machines/uuid-2", r.URL.Path)
		}
		writeMachine(w, http.StatusOK, want)
	})

	got, err := client.GetMachine(context.Background(), "uuid-2")
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil machine, got nil")
	}
	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
}

func TestClient_GetMachine_NotFound(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	got, err := client.GetMachine(context.Background(), "missing-id")
	if err != nil {
		t.Fatalf("expected nil error on 404, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil machine on 404, got %+v", got)
	}
}

func TestClient_GetMachine_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetMachine(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

// --- UpdateMachine ---

func TestClient_UpdateMachine_Success(t *testing.T) {
	want := apiclient.Machine{ID: "uuid-3", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS923+", RAMGB: 8}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method: got %q, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/machines/uuid-3" {
			t.Errorf("path: got %q, want /api/v1/machines/uuid-3", r.URL.Path)
		}
		writeMachine(w, http.StatusOK, want)
	})

	got, err := client.UpdateMachine(context.Background(), apiclient.Machine{ID: "uuid-3", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS923+", RAMGB: 8})
	if err != nil {
		t.Fatalf("UpdateMachine: %v", err)
	}
	if got.Model != want.Model {
		t.Errorf("Model: got %q, want %q", got.Model, want.Model)
	}
	if got.RAMGB != want.RAMGB {
		t.Errorf("RAMGB: got %d, want %d", got.RAMGB, want.RAMGB)
	}
}

func TestClient_UpdateMachine_NotFound(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.UpdateMachine(context.Background(), apiclient.Machine{ID: "ghost"})
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestClient_UpdateMachine_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.UpdateMachine(context.Background(), apiclient.Machine{ID: "some-id"})
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

// --- DeleteMachine ---

func TestClient_DeleteMachine_Success(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: got %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/machines/uuid-4" {
			t.Errorf("path: got %q, want /api/v1/machines/uuid-4", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteMachine(context.Background(), "uuid-4"); err != nil {
		t.Fatalf("DeleteMachine: %v", err)
	}
}

func TestClient_DeleteMachine_NotFound(t *testing.T) {
	// 404 on delete should be treated as a success (already gone).
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := client.DeleteMachine(context.Background(), "missing-id"); err != nil {
		t.Fatalf("expected nil error on 404, got: %v", err)
	}
}

func TestClient_DeleteMachine_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := client.DeleteMachine(context.Background(), "some-id")
	if err == nil {
		t.Fatal("expected error on unexpected status code, got nil")
	}
}

// --- Auth header ---

func TestClient_SendsBearerToken(t *testing.T) {
	var gotAuth string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	})

	_ = client.DeleteMachine(context.Background(), "any-id")
	if gotAuth != "Bearer "+testToken {
		t.Errorf("Authorization header: got %q, want %q", gotAuth, "Bearer "+testToken)
	}
}

// --- ListMachines ---

func TestClient_ListMachines_All(t *testing.T) {
	machines := []apiclient.Machine{
		{ID: "uuid-1", Name: "pve1", Kind: "proxmox", Make: "Dell", Model: "R640"},
		{ID: "uuid-2", Name: "nas01", Kind: "nas", Make: "Synology", Model: "DS920+"},
	}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/machines" {
			t.Errorf("path: got %q, want /api/v1/machines", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(machines)
	})

	got, err := client.ListMachines(context.Background(), "")
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].Name != "pve1" {
		t.Errorf("machines[0].Name: got %q, want %q", got[0].Name, "pve1")
	}
}

func TestClient_ListMachines_WithKindFilter(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "kind=proxmox" {
			t.Errorf("query: got %q, want %q", r.URL.RawQuery, "kind=proxmox")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]apiclient.Machine{
			{ID: "uuid-1", Name: "pve1", Kind: "proxmox", Make: "Dell", Model: "R640"},
		})
	})

	got, err := client.ListMachines(context.Background(), "proxmox")
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0].Kind != "proxmox" {
		t.Errorf("Kind: got %q, want proxmox", got[0].Kind)
	}
}

func TestClient_ListMachines_Empty(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]apiclient.Machine{})
	})

	got, err := client.ListMachines(context.Background(), "")
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got))
	}
}

func TestClient_ListMachines_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.ListMachines(context.Background(), "")
	if err == nil {
		t.Fatal("expected error on non-200 response, got nil")
	}
}

// --- Content-Type ---

func TestClient_SetsContentTypeOnWrite(t *testing.T) {
	var gotCT string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		writeMachine(w, http.StatusCreated, apiclient.Machine{ID: "x"})
	})

	_, _ = client.CreateMachine(context.Background(), apiclient.Machine{Name: "n", Kind: "nas", Make: "m", Model: "m"})
	if gotCT != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", gotCT)
	}
}
