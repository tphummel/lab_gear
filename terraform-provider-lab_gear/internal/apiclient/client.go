package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client is an HTTP client for the lab_gear REST API.
type Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// NewClient creates a Client targeting endpoint with Bearer token auth.
func NewClient(endpoint, token string) *Client {
	return &Client{
		endpoint:   endpoint,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Machine mirrors the JSON shape of the lab_gear service API.
type Machine struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Make      string  `json:"make"`
	Model     string  `json:"model"`
	CPU       string  `json:"cpu"`
	RAMGB     int64   `json:"ram_gb"`
	StorageTB float64 `json:"storage_tb"`
	Location  string  `json:"location"`
	Serial    string  `json:"serial"`
	Notes     string  `json:"notes"`
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// CreateMachine POSTs a new machine and returns the server-assigned record.
func (c *Client) CreateMachine(ctx context.Context, m Machine) (*Machine, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/machines", m)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create machine: unexpected status %d", resp.StatusCode)
	}
	var out Machine
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// GetMachine fetches a single machine by ID. Returns nil, nil when the server
// responds 404 so callers can treat a missing machine as "removed externally".
func (c *Client) GetMachine(ctx context.Context, id string) (*Machine, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/machines/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get machine %q: unexpected status %d", id, resp.StatusCode)
	}
	var out Machine
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// UpdateMachine PUTs a full replacement for the machine with m.ID.
func (c *Client) UpdateMachine(ctx context.Context, m Machine) (*Machine, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, "/api/v1/machines/"+m.ID, m)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("update machine %q: not found", m.ID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update machine %q: unexpected status %d", m.ID, resp.StatusCode)
	}
	var out Machine
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// DeleteMachine removes the machine with the given ID.
func (c *Client) DeleteMachine(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/machines/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("delete machine %q: unexpected status %d", id, resp.StatusCode)
}
