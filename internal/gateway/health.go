package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
)

// HealthStatus is the JSON body returned by /health.
type HealthStatus struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

// HealthHandler checks every downstream service concurrently (via errgroup,
// each check bounded by a 5s timeout derived from the request context) and
// reports per-service status. It never fails the whole response just
// because one service is down — it reports "degraded" with details instead.
func HealthHandler(users *userclient.Client, posts *postclient.Client, comments *commentclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var mu sync.Mutex
		results := make(map[string]error, 3)

		g, gctx := errgroup.WithContext(ctx)
		check := func(name string, fn func(context.Context) error) {
			g.Go(func() error {
				err := fn(gctx)
				mu.Lock()
				results[name] = err
				mu.Unlock()
				return nil // never let one service's failure short-circuit the others
			})
		}

		check("user-service", users.Healthy)
		check("post-service", posts.Healthy)
		check("comment-service", comments.Healthy)
		_ = g.Wait()

		status := HealthStatus{Status: "ok", Services: make(map[string]string, 3)}
		for name, err := range results {
			if err != nil {
				status.Status = "degraded"
				status.Services[name] = err.Error()
			} else {
				status.Services[name] = "ok"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if status.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(status)
	}
}
