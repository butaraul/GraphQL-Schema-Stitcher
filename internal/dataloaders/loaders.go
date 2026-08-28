package dataloaders

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
	"graphql-gateway/internal/models"
)

// Loaders bundles every dataloader the resolvers need. One instance is
// created per incoming HTTP request by Middleware, so batching/caching never
// crosses request boundaries.
type Loaders struct {
	UserByID       *Loader[string, *models.User]
	PostByID       *Loader[string, *models.Post]
	PostsByUserID  *Loader[string, []*models.Post]
	CommentByID    *Loader[string, *models.Comment]
	CommentsByPost *Loader[string, []*models.Comment]
}

type ctxKey struct{}

// New builds a fresh set of loaders backed by the given downstream clients.
// Each loader's batch function logs and gracefully degrades (per-key nil
// error, or a per-key error routed back to the caller) rather than letting
// one bad key or a downstream outage take down an entire batch.
func New(users *userclient.Client, posts *postclient.Client, comments *commentclient.Client, log *slog.Logger) *Loaders {
	return &Loaders{
		UserByID: NewLoader(func(ctx context.Context, ids []string) (map[string]*models.User, map[string]error) {
			values := make(map[string]*models.User, len(ids))
			errs := make(map[string]error, len(ids))

			found, err := users.GetUsers(ctx, ids)
			if err != nil {
				log.Error("user service batch fetch failed", "ids", ids, "error", err)
				for _, id := range ids {
					errs[id] = fmt.Errorf("user service unavailable: %w", err)
				}
				return values, errs
			}
			for _, id := range ids {
				if u, ok := found[id]; ok {
					values[id] = u
				} else {
					errs[id] = fmt.Errorf("user %s not found", id)
				}
			}
			return values, errs
		}),

		PostByID: NewLoader(func(ctx context.Context, ids []string) (map[string]*models.Post, map[string]error) {
			values := make(map[string]*models.Post, len(ids))
			errs := make(map[string]error, len(ids))

			found, err := posts.GetPosts(ctx, ids)
			if err != nil {
				log.Error("post service batch fetch failed", "ids", ids, "error", err)
				for _, id := range ids {
					errs[id] = fmt.Errorf("post service unavailable: %w", err)
				}
				return values, errs
			}
			for _, id := range ids {
				if p, ok := found[id]; ok {
					values[id] = p
				} else {
					errs[id] = fmt.Errorf("post %s not found", id)
				}
			}
			return values, errs
		}),

		PostsByUserID: NewLoader(func(ctx context.Context, userIDs []string) (map[string][]*models.Post, map[string]error) {
			values := make(map[string][]*models.Post, len(userIDs))
			errs := make(map[string]error, len(userIDs))

			found, err := posts.GetPostsByUsers(ctx, userIDs)
			if err != nil {
				log.Error("post service by-user batch fetch failed", "userIds", userIDs, "error", err)
				for _, id := range userIDs {
					errs[id] = fmt.Errorf("post service unavailable: %w", err)
				}
				return values, errs
			}
			for _, id := range userIDs {
				values[id] = found[id] // nil/empty slice is a valid "no posts" result, not an error
			}
			return values, errs
		}),

		CommentByID: NewLoader(func(ctx context.Context, ids []string) (map[string]*models.Comment, map[string]error) {
			values := make(map[string]*models.Comment, len(ids))
			errs := make(map[string]error, len(ids))

			found, err := comments.GetComments(ctx, ids)
			if err != nil {
				log.Error("comment service batch fetch failed", "ids", ids, "error", err)
				for _, id := range ids {
					errs[id] = fmt.Errorf("comment service unavailable: %w", err)
				}
				return values, errs
			}
			for _, id := range ids {
				if c, ok := found[id]; ok {
					values[id] = c
				} else {
					errs[id] = fmt.Errorf("comment %s not found", id)
				}
			}
			return values, errs
		}),

		CommentsByPost: NewLoader(func(ctx context.Context, postIDs []string) (map[string][]*models.Comment, map[string]error) {
			values := make(map[string][]*models.Comment, len(postIDs))
			errs := make(map[string]error, len(postIDs))

			found, err := comments.GetCommentsByPosts(ctx, postIDs)
			if err != nil {
				log.Error("comment service by-post batch fetch failed", "postIds", postIDs, "error", err)
				for _, id := range postIDs {
					errs[id] = fmt.Errorf("comment service unavailable: %w", err)
				}
				return values, errs
			}
			for _, id := range postIDs {
				values[id] = found[id]
			}
			return values, errs
		}),
	}
}

// Middleware injects a fresh set of Loaders into the request context so
// resolvers can pull them out with For(ctx). One set per HTTP request keeps
// batching/caching scoped correctly.
func Middleware(users *userclient.Client, posts *postclient.Client, comments *commentclient.Client, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			loaders := New(users, posts, comments, log)
			ctx := context.WithValue(r.Context(), ctxKey{}, loaders)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// For retrieves the Loaders stashed in ctx by Middleware. It panics if none
// are present, which would indicate the middleware was not wired up — a
// programmer error, not a runtime condition to recover from gracefully.
func For(ctx context.Context) *Loaders {
	loaders, ok := ctx.Value(ctxKey{}).(*Loaders)
	if !ok {
		panic("dataloaders: no Loaders in context; is dataloaders.Middleware wired up?")
	}
	return loaders
}
