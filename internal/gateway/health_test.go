package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
	"graphql-gateway/internal/gateway"
)

func healthzServer(t *testing.T, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// TestHealthHandler_ChecksAllThreeConcurrently verifies the 3 downstream
// health checks run in parallel (total time ~= one check's delay, not the
// sum of all three) and that all three servers are actually hit.
func TestHealthHandler_ChecksAllThreeConcurrently(t *testing.T) {
	const delay = 100 * time.Millisecond

	userSrv, userHits := healthzServer(t, delay)
	postSrv, postHits := healthzServer(t, delay)
	commentSrv, commentHits := healthzServer(t, delay)

	users := userclient.New(userSrv.URL)
	posts := postclient.New(postSrv.URL)
	comments := commentclient.New(commentSrv.URL)

	h := gateway.HealthHandler(users, posts, comments)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if elapsed > 3*delay {
		t.Errorf("expected concurrent checks to take ~%v, took %v (looks sequential)", delay, elapsed)
	}
	if userHits.Load() != 1 || postHits.Load() != 1 || commentHits.Load() != 1 {
		t.Errorf("expected each service hit exactly once, got user=%d post=%d comment=%d",
			userHits.Load(), postHits.Load(), commentHits.Load())
	}

	var status gateway.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if status.Status != "ok" {
		t.Errorf("expected status ok, got %q (services: %v)", status.Status, status.Services)
	}
}

// TestHealthHandler_DegradedWhenOneServiceIsDown verifies one unreachable
// service is reported without the handler failing outright.
func TestHealthHandler_DegradedWhenOneServiceIsDown(t *testing.T) {
	userSrv, _ := healthzServer(t, 0)
	postSrv, _ := healthzServer(t, 0)

	deadSrv := httptest.NewServer(nil)
	deadSrv.Close()

	users := userclient.New(userSrv.URL)
	posts := postclient.New(postSrv.URL)
	comments := commentclient.New(deadSrv.URL)

	h := gateway.HealthHandler(users, posts, comments)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}

	var status gateway.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if status.Status != "degraded" {
		t.Errorf("expected status degraded, got %q", status.Status)
	}
	if status.Services["user-service"] != "ok" || status.Services["post-service"] != "ok" {
		t.Errorf("expected user/post services ok, got: %v", status.Services)
	}
	if status.Services["comment-service"] == "ok" || status.Services["comment-service"] == "" {
		t.Errorf("expected comment-service to report an error, got: %v", status.Services)
	}
}
