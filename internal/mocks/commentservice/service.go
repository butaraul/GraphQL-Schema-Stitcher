// Package commentservice is a mock stand-in for the real Comment Service
// GraphQL microservice. See internal/mocks/userservice for why it speaks
// plain JSON.
package commentservice

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"graphql-gateway/internal/mocks"
	"graphql-gateway/internal/models"
)

// Service is the mock Comment Service.
type Service struct {
	log *slog.Logger

	// BatchCalls / ByPostCalls count hits to each endpoint — useful in tests
	// to assert that the gateway's dataloaders actually coalesced requests.
	BatchCalls  atomic.Int64
	ByPostCalls atomic.Int64
}

// New creates a mock Comment Service.
func New(log *slog.Logger) *Service {
	return &Service{log: log}
}

// Handler returns the HTTP handler for the mock service.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /batch", s.handleBatch)
	mux.HandleFunc("POST /by-post", s.handleByPost)
	return mux
}

func (s *Service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type batchRequest struct {
	IDs []string `json:"ids"`
}

type batchResponse struct {
	Comments map[string]*models.Comment `json:"comments"`
}

func (s *Service) handleBatch(w http.ResponseWriter, r *http.Request) {
	s.BatchCalls.Add(1)

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Info("comment service: batch fetch by id", "count", len(req.IDs), "ids", req.IDs)

	out := make(map[string]*models.Comment, len(req.IDs))
	for _, id := range req.IDs {
		if c, ok := mocks.Comments[id]; ok {
			out[id] = c
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batchResponse{Comments: out})
}

type byPostRequest struct {
	PostIDs []string `json:"postIds"`
}

type byPostResponse struct {
	Comments map[string][]*models.Comment `json:"comments"`
}

func (s *Service) handleByPost(w http.ResponseWriter, r *http.Request) {
	s.ByPostCalls.Add(1)

	var req byPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Info("comment service: batch fetch by post", "count", len(req.PostIDs), "postIds", req.PostIDs)

	out := make(map[string][]*models.Comment, len(req.PostIDs))
	for _, postID := range req.PostIDs {
		out[postID] = mocks.CommentsByPost(postID)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(byPostResponse{Comments: out})
}
