// Command gateway runs the GraphQL schema-stitching gateway. It exposes a
// single /graphql endpoint (with a playground at /) that federates the User,
// Post, and Comment services, plus a /health endpoint.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
	"graphql-gateway/internal/dataloaders"
	"graphql-gateway/internal/gateway"
	"graphql-gateway/internal/generated"
	"graphql-gateway/internal/resolvers"
)

// requestTimeout bounds the total time any single gateway request may run,
// including however many downstream round trips its resolvers make.
const requestTimeout = 5 * time.Second

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("service", "gateway")

	addr := getenv("GATEWAY_ADDR", ":8080")
	userServiceURL := getenv("USER_SERVICE_URL", "http://localhost:8081")
	postServiceURL := getenv("POST_SERVICE_URL", "http://localhost:8082")
	commentServiceURL := getenv("COMMENT_SERVICE_URL", "http://localhost:8083")

	users := userclient.New(userServiceURL)
	posts := postclient.New(postServiceURL)
	comments := commentclient.New(commentServiceURL)

	resolver := &resolvers.Resolver{Users: users, Posts: posts, Comments: comments, Log: log}
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: resolver})
	graphqlSrv := handler.NewDefaultServer(schema)

	mux := http.NewServeMux()
	mux.Handle("/graphql", dataloaders.Middleware(users, posts, comments, log)(graphqlSrv))
	mux.Handle("/health", gateway.HealthHandler(users, posts, comments))
	mux.Handle("/", playground.Handler("GraphQL Gateway Playground", "/graphql"))

	root := gateway.TimeoutMiddleware(requestTimeout)(mux)

	log.Info("gateway starting",
		"addr", addr,
		"userService", userServiceURL,
		"postService", postServiceURL,
		"commentService", commentServiceURL,
	)

	if err := gateway.Run(addr, root, log, 10*time.Second); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
