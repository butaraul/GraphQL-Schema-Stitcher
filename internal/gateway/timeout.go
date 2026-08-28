package gateway

import (
	"context"
	"net/http"
	"time"
)

// TimeoutMiddleware bounds every request's context to d, so a slow or
// hanging downstream call can't hold a gateway request open forever.
func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
