package dataloaders_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
	"graphql-gateway/internal/dataloaders"
	"graphql-gateway/internal/mocks/commentservice"
	"graphql-gateway/internal/mocks/postservice"
	"graphql-gateway/internal/mocks/userservice"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestLoaders_BatchAcrossConcurrentResolvers is the integration-level
// analogue of TestLoader_Batching: it drives the loaders the way GraphQL
// field resolvers actually would — many goroutines, each resolving one
// object's relation — against real (mock) HTTP services, and asserts the
// downstream services only see one batched round trip.
func TestLoaders_BatchAcrossConcurrentResolvers(t *testing.T) {
	log := discardLogger()

	userSvc := userservice.New(log)
	userSrv := httptest.NewServer(userSvc.Handler())
	defer userSrv.Close()

	postSvc := postservice.New(log)
	postSrv := httptest.NewServer(postSvc.Handler())
	defer postSrv.Close()

	commentSvc := commentservice.New(log)
	commentSrv := httptest.NewServer(commentSvc.Handler())
	defer commentSrv.Close()

	users := userclient.New(userSrv.URL)
	posts := postclient.New(postSrv.URL)
	comments := commentclient.New(commentSrv.URL)

	loaders := dataloaders.New(users, posts, comments, log)
	ctx := context.Background()

	// Simulate resolving Post.user for 4 posts authored by only 2 distinct
	// users (posts 101,103 -> user 1; 102,104 -> user 2 in effect, but here
	// we just fan out over known seeded user IDs "1","2","3") concurrently,
	// as gqlgen would do for sibling fields in a selection set.
	ids := []string{"1", "2", "3", "1", "2", "3", "1", "2"}
	var wg sync.WaitGroup
	wg.Add(len(ids))
	for _, id := range ids {
		go func(id string) {
			defer wg.Done()
			u, err := loaders.UserByID.Load(ctx, id)
			if err != nil {
				t.Errorf("load user %s: %v", id, err)
				return
			}
			if u == nil || u.ID != id {
				t.Errorf("load user %s: got %+v", id, u)
			}
		}(id)
	}
	wg.Wait()

	if got := userSvc.BatchCalls.Load(); got != 1 {
		t.Errorf("expected user service /batch to be hit exactly once, got %d calls", got)
	}

	// Same idea for PostsByUserID (backs User.posts).
	userIDs := []string{"1", "2", "3", "1", "2", "3"}
	wg.Add(len(userIDs))
	for _, id := range userIDs {
		go func(id string) {
			defer wg.Done()
			if _, err := loaders.PostsByUserID.Load(ctx, id); err != nil {
				t.Errorf("load posts for user %s: %v", id, err)
			}
		}(id)
	}
	wg.Wait()

	if got := postSvc.ByUserCalls.Load(); got != 1 {
		t.Errorf("expected post service /by-user to be hit exactly once, got %d calls", got)
	}

	// And CommentsByPost (backs Post.comments).
	postIDs := []string{"101", "102", "103", "104"}
	wg.Add(len(postIDs))
	for _, id := range postIDs {
		go func(id string) {
			defer wg.Done()
			if _, err := loaders.CommentsByPost.Load(ctx, id); err != nil {
				t.Errorf("load comments for post %s: %v", id, err)
			}
		}(id)
	}
	wg.Wait()

	if got := commentSvc.ByPostCalls.Load(); got != 1 {
		t.Errorf("expected comment service /by-post to be hit exactly once, got %d calls", got)
	}
}

// TestLoaders_DownstreamFailureIsPerKeyGraceful verifies that when a
// downstream service is unreachable, the loader reports an error for the
// affected keys instead of panicking or hanging.
func TestLoaders_DownstreamFailureIsPerKeyGraceful(t *testing.T) {
	log := discardLogger()

	// Point the user client at a server that immediately closes connections.
	deadSrv := httptest.NewServer(nil)
	deadSrv.Close() // closed before use -> connection refused

	users := userclient.New(deadSrv.URL)
	posts := postclient.New(deadSrv.URL)
	comments := commentclient.New(deadSrv.URL)

	loaders := dataloaders.New(users, posts, comments, log)
	ctx := context.Background()

	_, err := loaders.UserByID.Load(ctx, "1")
	if err == nil {
		t.Fatal("expected an error when the user service is unreachable, got nil")
	}
}
