// Package userclient is an HTTP client for the User Service.
package userclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"graphql-gateway/internal/models"
)

// DefaultTimeout is the per-request timeout applied to every downstream call.
const DefaultTimeout = 5 * time.Second

// Client talks to the User Service over HTTP.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New creates a User Service client pointed at baseURL (e.g. http://localhost:8081).
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

type batchRequest struct {
	IDs []string `json:"ids"`
}

type batchResponse struct {
	Users map[string]*models.User `json:"users"`
}

// GetUsers fetches every user in ids in a single round trip. Missing IDs are
// simply absent from the result map; the caller decides how to handle that.
func (c *Client) GetUsers(ctx context.Context, ids []string) (map[string]*models.User, error) {
	if len(ids) == 0 {
		return map[string]*models.User{}, nil
	}

	body, err := json.Marshal(batchRequest{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("userclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/batch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("userclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userclient: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userclient: unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var out batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("userclient: decode response: %w", err)
	}
	return out.Users, nil
}

// GetUser fetches a single user by ID.
func (c *Client) GetUser(ctx context.Context, id string) (*models.User, error) {
	users, err := c.GetUsers(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return users[id], nil
}

// Healthy checks that the User Service is reachable.
func (c *Client) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("userclient: unhealthy status %d", resp.StatusCode)
	}
	return nil
}
