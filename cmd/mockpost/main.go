// Command mockpost runs the mock Post Service used for local development
// and tests. It listens on :8082 by default.
package main

import (
	"log/slog"
	"os"
	"time"

	"graphql-gateway/internal/gateway"
	"graphql-gateway/internal/mocks/postservice"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "mock-post")

	addr := os.Getenv("POST_SERVICE_ADDR")
	if addr == "" {
		addr = ":8082"
	}

	svc := postservice.New(log)
	if err := gateway.Run(addr, svc.Handler(), log, 5*time.Second); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
