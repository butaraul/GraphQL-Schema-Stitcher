// Package userservice is a mock stand-in for the real User Service GraphQL
// microservice. It speaks plain JSON over HTTP so the gateway's tests and
// `make run` demo don't need a second GraphQL server stack just to exercise
// the gateway's stitching and dataloader logic.
package userservice

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"graphql-gateway/internal/mocks"
	"graphql-gateway/internal/models"
)

// Service is the mock User Service.
type Service struct {
	log *slog.Logger

	// BatchCalls counts how many times /batch has been hit — useful in tests
	// to assert that the gateway's dataloader actually coalesced requests.
	BatchCalls atomic.Int64
}

// New creates a mock User Service.
func New(log *slog.Logger) *Service {
	return &Service{log: log}
}

// Handler returns the HTTP handler for the mock service.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /batch", s.handleBatch)
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
	Users map[string]*models.User `json:"users"`
}

func (s *Service) handleBatch(w http.ResponseWriter, r *http.Request) {
	s.BatchCalls.Add(1)

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.log.Info("user service: batch fetch", "count", len(req.IDs), "ids", req.IDs)

	out := make(map[string]*models.User, len(req.IDs))
	for _, id := range req.IDs {
		if u, ok := mocks.Users[id]; ok {
			out[id] = u
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(batchResponse{Users: out})
}
