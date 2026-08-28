// Command mockcomment runs the mock Comment Service used for local
// development and tests. It listens on :8083 by default.
package main

import (
	"log/slog"
	"os"
	"time"

	"graphql-gateway/internal/gateway"
	"graphql-gateway/internal/mocks/commentservice"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "mock-comment")

	addr := os.Getenv("COMMENT_SERVICE_ADDR")
	if addr == "" {
		addr = ":8083"
	}

	svc := commentservice.New(log)
	if err := gateway.Run(addr, svc.Handler(), log, 5*time.Second); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
