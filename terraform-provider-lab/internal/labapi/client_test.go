package labapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientValidationAndNormalization(t *testing.T) {
	t.Run("rejects empty endpoint", func(t *testing.T) {
		_, err := NewClient("   ", "token")
		if err == nil {
			t.Fatal("expected error for empty endpoint")
		}
	})

	t.Run("trims trailing slash", func(t *testing.T) {
		c, err := NewClient("https://assets.lab.local/", "token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.baseURL != "https://assets.lab.local" {
			t.Fatalf("expected normalized base URL, got %q", c.baseURL)
		}
	})
}

func TestClientCreateMachineRequestAndResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/machines" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token123" {
			t.Fatalf("unexpected authorization header: %q", auth)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("unexpected accept header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content-type: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		defer r.Body.Close()

		var payload Machine
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload.Name != "pve2" || payload.Kind != "proxmox" {
			t.Fatalf("unexpected payload: %+v", payload)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc","name":"pve2","kind":"proxmox","make":"Dell","model":"R730"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token123")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	out, err := client.CreateMachine(context.Background(), Machine{Name: "pve2", Kind: "proxmox", Make: "Dell", Model: "R730"})
	if err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if out.ID != "abc" {
		t.Fatalf("expected ID abc, got %s", out.ID)
	}
}

func TestClientGetMachineNotFoundReturnsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.GetMachine(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "not found") {
		t.Fatalf("expected response body in error, got %q", apiErr.Body)
	}
}

func TestClientUpdateMachine(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/machines/id-123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"id-123","name":"pve2","kind":"proxmox","make":"Dell","model":"R740"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	out, err := client.UpdateMachine(context.Background(), "id-123", Machine{Name: "pve2", Kind: "proxmox", Make: "Dell", Model: "R740"})
	if err != nil {
		t.Fatalf("update machine: %v", err)
	}
	if out.Model != "R740" {
		t.Fatalf("expected updated model, got %q", out.Model)
	}
}

func TestClientDeleteMachineExpectedNoContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if err := client.DeleteMachine(context.Background(), "id-123"); err != nil {
		t.Fatalf("delete machine: %v", err)
	}
}

func TestDoJSONUnexpectedStatusAndInvalidJSON(t *testing.T) {
	t.Parallel()

	t.Run("unexpected status returns APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "token")
		if err != nil {
			t.Fatalf("new client: %v", err)
		}

		_, err = client.GetMachine(context.Background(), "id-123")
		if err == nil {
			t.Fatal("expected error")
		}
		apiErr, ok := err.(APIError)
		if !ok {
			t.Fatalf("expected APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", apiErr.StatusCode)
		}
	})

	t.Run("invalid json returns decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{invalid"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "token")
		if err != nil {
			t.Fatalf("new client: %v", err)
		}

		_, err = client.GetMachine(context.Background(), "id-123")
		if err == nil {
			t.Fatal("expected decode error")
		}
	})
}
