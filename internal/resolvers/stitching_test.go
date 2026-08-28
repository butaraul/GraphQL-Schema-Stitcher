package resolvers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	gqlhandler "github.com/99designs/gqlgen/graphql/handler"

	"graphql-gateway/internal/clients/commentclient"
	"graphql-gateway/internal/clients/postclient"
	"graphql-gateway/internal/clients/userclient"
	"graphql-gateway/internal/dataloaders"
	"graphql-gateway/internal/generated"
	"graphql-gateway/internal/mocks/commentservice"
	"graphql-gateway/internal/mocks/postservice"
	"graphql-gateway/internal/mocks/userservice"
	"graphql-gateway/internal/resolvers"
)

// testGateway wires up real mock downstream services and the actual GraphQL
// handler exactly as cmd/gateway/main.go does, so these tests exercise the
// genuine schema-stitching path end to end.
type testGateway struct {
	handler    http.Handler
	userSrv    *userservice.Service
	postSrv    *postservice.Service
	commentSrv *commentservice.Service
	closers    []func()
}

func newTestGateway(t *testing.T) *testGateway {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	userSvc := userservice.New(log)
	userHTTP := httptest.NewServer(userSvc.Handler())

	postSvc := postservice.New(log)
	postHTTP := httptest.NewServer(postSvc.Handler())

	commentSvc := commentservice.New(log)
	commentHTTP := httptest.NewServer(commentSvc.Handler())

	users := userclient.New(userHTTP.URL)
	posts := postclient.New(postHTTP.URL)
	comments := commentclient.New(commentHTTP.URL)

	resolver := &resolvers.Resolver{Users: users, Posts: posts, Comments: comments, Log: log}
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: resolver})
	gqlSrv := gqlhandler.NewDefaultServer(schema)

	h := dataloaders.Middleware(users, posts, comments, log)(gqlSrv)

	tg := &testGateway{
		handler:    h,
		userSrv:    userSvc,
		postSrv:    postSvc,
		commentSrv: commentSvc,
		closers:    []func(){userHTTP.Close, postHTTP.Close, commentHTTP.Close},
	}
	t.Cleanup(func() {
		for _, c := range tg.closers {
			c()
		}
	})
	return tg
}

func (tg *testGateway) query(t *testing.T, query string) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	tg.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return out
}

// TestStitching_NestedRelationsResolveCorrectly is the schema-stitching
// test: it walks User -> Post -> Comment -> author (back to User) in one
// query and checks every hop links to the right seeded entity.
func TestStitching_NestedRelationsResolveCorrectly(t *testing.T) {
	tg := newTestGateway(t)

	resp := tg.query(t, `
		query {
			user(id: "1") {
				id
				name
				posts {
					id
					title
					user { id name }
					comments {
						id
						text
						author { id name }
					}
				}
			}
		}
	`)

	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data in response: %v", resp)
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("missing user: %v", data)
	}
	if user["id"] != "1" || user["name"] != "Ada Lovelace" {
		t.Errorf("unexpected user: %v", user)
	}

	postsRaw, ok := user["posts"].([]any)
	if !ok || len(postsRaw) == 0 {
		t.Fatalf("expected at least one post for user 1, got: %v", user["posts"])
	}

	var post101 map[string]any
	for _, p := range postsRaw {
		post := p.(map[string]any)
		if post["id"] == "101" {
			post101 = post
		}
		// Post.user must point back to the same user we started from.
		nestedUser := post["user"].(map[string]any)
		if nestedUser["id"] != "1" {
			t.Errorf("post %v: expected user 1, got %v", post["id"], nestedUser)
		}
	}
	if post101 == nil {
		t.Fatalf("expected seeded post 101 among user 1's posts, got: %v", postsRaw)
	}

	commentsRaw := post101["comments"].([]any)
	if len(commentsRaw) != 2 {
		t.Fatalf("expected 2 seeded comments on post 101, got %d: %v", len(commentsRaw), commentsRaw)
	}

	seenAuthors := map[string]bool{}
	for _, c := range commentsRaw {
		comment := c.(map[string]any)
		author := comment["author"].(map[string]any)
		seenAuthors[author["id"].(string)] = true
	}
	if !seenAuthors["2"] || !seenAuthors["3"] {
		t.Errorf("expected comments on post 101 authored by users 2 and 3, got authors: %v", seenAuthors)
	}
}

// TestStitching_ConcurrentSiblingFields verifies a query that asks for
// user + posts + comments as independent top-level fields (which gqlgen
// resolves concurrently) all resolve correctly in a single round trip.
func TestStitching_ConcurrentSiblingFields(t *testing.T) {
	tg := newTestGateway(t)

	resp := tg.query(t, `
		query {
			user(id: "1") { id name }
			posts(userId: "3") { id title }
			comments(postId: "103") { id text }
		}
	`)

	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data := resp["data"].(map[string]any)

	if data["user"].(map[string]any)["id"] != "1" {
		t.Errorf("unexpected user: %v", data["user"])
	}
	if len(data["posts"].([]any)) != 2 {
		t.Errorf("expected 2 posts for user 3, got: %v", data["posts"])
	}
	if len(data["comments"].([]any)) != 2 {
		t.Errorf("expected 2 comments for post 103, got: %v", data["comments"])
	}
}

// TestStitching_DownstreamFailureReturnsPartialResult verifies requirement
// #5: when a downstream service is down, the affected field comes back null
// with a GraphQL error attached, while the rest of the response (and the
// gateway process) is unaffected.
func TestStitching_DownstreamFailureReturnsPartialResult(t *testing.T) {
	tg := newTestGateway(t)

	// Kill the comment service mid-test to simulate an outage.
	tg.closers[2]()

	resp := tg.query(t, `
		query {
			user(id: "1") { id name }
			post(id: "101") {
				id
				comments { id text }
			}
		}
	`)

	if _, ok := resp["errors"]; !ok {
		t.Fatalf("expected errors for the failed comment service, got none: %v", resp)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected a data object alongside errors, got: %v", resp)
	}

	// The unrelated `user` field must still have resolved successfully.
	user, ok := data["user"].(map[string]any)
	if !ok || user["id"] != "1" {
		t.Errorf("expected user field to still resolve despite comment service outage, got: %v", data["user"])
	}

	// `post` is non-null and contains a non-null `comments` list, so per
	// GraphQL null-propagation the failure of `comments` nulls out `post`
	// itself — but the response as a whole is still well-formed (HTTP 200,
	// partial data, errors array), not a crash.
	if data["post"] != nil {
		t.Errorf("expected post to be null due to comments failure, got: %v", data["post"])
	}
}

// TestStitching_UnknownIDsAreOmittedNotFatal verifies Query.users(ids) skips
// unresolvable IDs (reporting an error for them) instead of failing the
// entire list, per requirement #5.
func TestStitching_UnknownIDsAreOmittedNotFatal(t *testing.T) {
	tg := newTestGateway(t)

	resp := tg.query(t, `
		query {
			users(ids: ["1", "does-not-exist", "2"]) { id name }
		}
	`)

	data := resp["data"].(map[string]any)
	users := data["users"].([]any)
	if len(users) != 2 {
		t.Fatalf("expected 2 resolvable users out of 3 requested, got %d: %v", len(users), users)
	}
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected an error entry for the unknown id, got none")
	}
}
