# GraphQL Gateway

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Made with Go](https://img.shields.io/badge/Made%20with-Go-00ADD8.svg)](https://golang.org/)
[![GraphQL](https://img.shields.io/badge/GraphQL-E10098.svg?logo=graphql&logoColor=white)](https://graphql.org/)
[![GitHub stars](https://img.shields.io/github/stars/butaraul/GraphQL-Schema-Stitcher.svg?style=social)](https://github.com/butaraul/GraphQL-Schema-Stitcher/stargazers)

A single GraphQL gateway (schema stitching) that federates three downstream
services — Users, Posts, Comments — into one unified graph, with dataloader
batching, concurrent field resolution, graceful error handling, and a health
check.

```
type User {
  id: ID!
  name: String!
  email: String!
  avatar: String!
  posts: [Post!]!
}

type Post {
  id: ID!
  title: String!
  content: String!
  createdAt: String!
  user: User!
  comments: [Comment!]!
}

type Comment {
  id: ID!
  text: String!
  createdAt: String!
  post: Post!
  author: User!
}
```

## Quick start

```bash
make run
```

This starts the mock User (`:8081`), Post (`:8082`), and Comment (`:8083`)
services, then the gateway (`:8080`). Open `http://localhost:8080` for the
GraphQL Playground.

```graphql
query {
  user(id: "1") {
    name
    posts {
      title
      comments {
        text
        author { name }
      }
    }
  }
}
```

```bash
curl http://localhost:8080/health
```

## Project layout

- `cmd/gateway` — the gateway binary
- `cmd/mockuser`, `cmd/mockpost`, `cmd/mockcomment` — standalone mock downstream services (seed data in `internal/mocks/data.go`)
- `internal/schema` — GraphQL SDL (`schema.graphqls`)
- `internal/generated` — gqlgen-generated exec/model code (do not edit by hand; regenerate with `make generate`)
- `internal/resolvers` — resolver implementations, wired to dataloaders and clients
- `internal/dataloaders` — generic per-request batching/caching loader + the concrete loaders for each relation
- `internal/clients` — HTTP clients for the three downstream services (5s timeout each)
- `internal/gateway` — graceful HTTP server runner, `/health` handler, request-timeout middleware

## Design notes

- **Batching**: each dataloader coalesces concurrent `Load` calls issued within the same tick into one downstream HTTP call (e.g. resolving `posts` for 3 different users in one query results in a single `POST /by-user` call with all 3 IDs).
- **Concurrency**: gqlgen resolves sibling fields in a selection set concurrently by default; the dataloaders make that concurrency land in shared batches instead of N separate downstream calls. `/health` checks all three services concurrently via `errgroup`.
- **Error handling**: a downstream failure is logged and returned as a per-field GraphQL error rather than crashing the gateway or failing the whole response — unrelated sibling fields still resolve. Note: since `posts`/`comments`/`user`/`author` are non-null in the schema, a failure there nulls the *parent* object per GraphQL's null-propagation rules — that's inherent to the schema, not a gap in error handling.
- **Timeouts**: every downstream HTTP call has a 5s timeout; every incoming gateway request is bounded to 5s total via middleware.

## Commands

| Command | Description |
|---|---|
| `make run` | Start all 3 mock services + the gateway |
| `make run-mocks` | Start only the 3 mock services |
| `make run-gateway` | Start only the gateway (expects mocks already running) |
| `make generate` | Regenerate gqlgen exec/model code from `internal/schema/*.graphqls` |
| `make test` | Run the full test suite with the race detector |
| `make build` | Compile all binaries into `./bin` |
| `make tidy` | `go mod tidy` |

## Tests

```bash
make test
```

Covers dataloader batching/caching/error handling/context cancellation
(`internal/dataloaders`), concurrent health checks (`internal/gateway`), and
end-to-end schema stitching including partial-failure behavior
(`internal/resolvers`).
