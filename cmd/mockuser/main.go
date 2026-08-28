// Command mockuser runs the mock User Service used for local development
// and tests. It listens on :8081 by default.
package main

import (
	"log/slog"
	"os"
	"time"

	"graphql-gateway/internal/gateway"
	"graphql-gateway/internal/mocks/userservice"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "mock-user")

	addr := os.Getenv("USER_SERVICE_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	svc := userservice.New(log)
	if err := gateway.Run(addr, svc.Handler(), log, 5*time.Second); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
