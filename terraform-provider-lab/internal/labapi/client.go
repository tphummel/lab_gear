package labapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Machine struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Make      string   `json:"make"`
	Model     string   `json:"model"`
	CPU       *string  `json:"cpu,omitempty"`
	RAMGB     *int64   `json:"ram_gb,omitempty"`
	StorageTB *float64 `json:"storage_tb,omitempty"`
	Location  *string  `json:"location,omitempty"`
	Serial    *string  `json:"serial,omitempty"`
	Notes     *string  `json:"notes,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e APIError) Error() string {
	return fmt.Sprintf("lab API returned status %d: %s", e.StatusCode, e.Body)
}

func NewClient(endpoint, apiKey string) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint cannot be empty")
	}

	return &Client{
		baseURL: strings.TrimRight(endpoint, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (c *Client) CreateMachine(ctx context.Context, machine Machine) (*Machine, error) {
	var out Machine
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/machines", machine, http.StatusCreated, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachine(ctx context.Context, id string) (*Machine, error) {
	var out Machine
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/machines/"+id, nil, http.StatusOK, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMachine(ctx context.Context, id string, machine Machine) (*Machine, error) {
	var out Machine
	err := c.doJSON(ctx, http.MethodPut, "/api/v1/machines/"+id, machine, http.StatusOK, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteMachine(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/machines/"+id, nil, http.StatusNoContent, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, expectedStatus int, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != expectedStatus {
		return APIError{StatusCode: resp.StatusCode, Body: string(payload)}
	}

	if out == nil || len(payload) == 0 {
		return nil
	}

	return json.Unmarshal(payload, out)
}
