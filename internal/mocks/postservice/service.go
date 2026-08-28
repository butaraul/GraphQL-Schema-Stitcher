// Package postservice is a mock stand-in for the real Post Service GraphQL
// microservice. See internal/mocks/userservice for why it speaks plain JSON.
package postservice

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"graphql-gateway/internal/mocks"
	"graphql-gateway/internal/models"
)

// Service is the mock Post Service.
type Service struct {
	log *slog.Logger

	// BatchCalls / ByUserCalls count hits to each endpoint — useful in tests
	// to assert that the gateway's dataloaders actually coalesced requests.
	BatchCalls  atomic.Int64
	ByUserCalls atomic.Int64
}

// New creates a mock Post Service.
func New(log *slog.Logger) *Service {
	return &Service{log: log}
}

// Handler returns the HTTP handler for the mock service.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /batch", s.handleBatch)
	mux.HandleFunc("POST /by-user", s.handleByUser)
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
	Posts map[string]*models.Post `json:"posts"`
}

func (s *Service) handleBatch(w http.ResponseWriter, r *http.Request) {
	s.BatchCalls.Add(1)

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Info("post service: batch fetch by id", "count", len(req.IDs), "ids", req.IDs)

	out := make(map[string]*models.Post, len(req.IDs))
	for _, id := range req.IDs {
		if p, ok := mocks.Posts[id]; ok {
			out[id] = p
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batchResponse{Posts: out})
}

type byUserRequest struct {
	UserIDs []string `json:"userIds"`
}

type byUserResponse struct {
	Posts map[string][]*models.Post `json:"posts"`
}

func (s *Service) handleByUser(w http.ResponseWriter, r *http.Request) {
	s.ByUserCalls.Add(1)

	var req byUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Info("post service: batch fetch by user", "count", len(req.UserIDs), "userIds", req.UserIDs)

	out := make(map[string][]*models.Post, len(req.UserIDs))
	for _, userID := range req.UserIDs {
		out[userID] = mocks.PostsByUser(userID)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(byUserResponse{Posts: out})
}
