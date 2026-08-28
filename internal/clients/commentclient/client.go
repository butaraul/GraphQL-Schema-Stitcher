// Package commentclient is an HTTP client for the Comment Service.
package commentclient

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

// Client talks to the Comment Service over HTTP.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New creates a Comment Service client pointed at baseURL (e.g. http://localhost:8083).
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
	Comments map[string]*models.Comment `json:"comments"`
}

// GetComments fetches every comment in ids in a single round trip.
func (c *Client) GetComments(ctx context.Context, ids []string) (map[string]*models.Comment, error) {
	if len(ids) == 0 {
		return map[string]*models.Comment{}, nil
	}
	var out byIDsResponse
	if err := c.post(ctx, "/batch", byIDsRequest{IDs: ids}, &out); err != nil {
		return nil, err
	}
	return out.Comments, nil
}

// GetComment fetches a single comment by ID.
func (c *Client) GetComment(ctx context.Context, id string) (*models.Comment, error) {
	comments, err := c.GetComments(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return comments[id], nil
}

type byPostRequest struct {
	PostIDs []string `json:"postIds"`
}

type byPostResponse struct {
	Comments map[string][]*models.Comment `json:"comments"`
}

// GetCommentsByPosts fetches, for each post ID, the list of comments on it,
// in a single round trip.
func (c *Client) GetCommentsByPosts(ctx context.Context, postIDs []string) (map[string][]*models.Comment, error) {
	if len(postIDs) == 0 {
		return map[string][]*models.Comment{}, nil
	}
	var out byPostResponse
	if err := c.post(ctx, "/by-post", byPostRequest{PostIDs: postIDs}, &out); err != nil {
		return nil, err
	}
	return out.Comments, nil
}

func (c *Client) post(ctx context.Context, path string, reqBody, respBody any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("commentclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("commentclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("commentclient: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("commentclient: unexpected status %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("commentclient: decode response: %w", err)
	}
	return nil
}

// Healthy checks that the Comment Service is reachable.
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
		return fmt.Errorf("commentclient: unhealthy status %d", resp.StatusCode)
	}
	return nil
}
