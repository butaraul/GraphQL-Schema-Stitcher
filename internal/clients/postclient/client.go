// Package postclient is an HTTP client for the Post Service.
package postclient

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

// Client talks to the Post Service over HTTP.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New creates a Post Service client pointed at baseURL (e.g. http://localhost:8082).
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

type byIDsRequest struct {
	IDs []string `json:"ids"`
}

type byIDsResponse struct {
	Posts map[string]*models.Post `json:"posts"`
}

// GetPosts fetches every post in ids in a single round trip.
func (c *Client) GetPosts(ctx context.Context, ids []string) (map[string]*models.Post, error) {
	if len(ids) == 0 {
		return map[string]*models.Post{}, nil
	}
	var out byIDsResponse
	if err := c.post(ctx, "/batch", byIDsRequest{IDs: ids}, &out); err != nil {
		return nil, err
	}
	return out.Posts, nil
}

// GetPost fetches a single post by ID.
func (c *Client) GetPost(ctx context.Context, id string) (*models.Post, error) {
	posts, err := c.GetPosts(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return posts[id], nil
}

type byUserRequest struct {
	UserIDs []string `json:"userIds"`
}

type byUserResponse struct {
	Posts map[string][]*models.Post `json:"posts"`
}

// GetPostsByUsers fetches, for each user ID, the list of posts they authored,
// in a single round trip.
func (c *Client) GetPostsByUsers(ctx context.Context, userIDs []string) (map[string][]*models.Post, error) {
	if len(userIDs) == 0 {
		return map[string][]*models.Post{}, nil
	}
	var out byUserResponse
	if err := c.post(ctx, "/by-user", byUserRequest{UserIDs: userIDs}, &out); err != nil {
		return nil, err
	}
	return out.Posts, nil
}

func (c *Client) post(ctx context.Context, path string, reqBody, respBody any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("postclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("postclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("postclient: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("postclient: unexpected status %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("postclient: decode response: %w", err)
	}
	return nil
}

// Healthy checks that the Post Service is reachable.
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
		return fmt.Errorf("postclient: unhealthy status %d", resp.StatusCode)
	}
	return nil
}
