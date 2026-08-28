// Package resolvers implements the GraphQL resolvers for the gateway. It
// stitches together the three downstream services (users, posts, comments)
// through internal/clients, batching repeated lookups via internal/dataloaders.
package resolvers

import (
	"log/slog"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
)

// Resolver is the root GraphQL resolver. It holds the downstream service
// clients used as a fallback / for dataloader batch functions; per-request
// data fetching for object fields goes through the dataloaders stashed in
// the request context by dataloaders.Middleware.
type Resolver struct {
	Users    *userclient.Client
	Posts    *postclient.Client
	Comments *commentclient.Client
	Log      *slog.Logger
}
