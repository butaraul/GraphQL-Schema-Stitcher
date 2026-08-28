// Package dataloaders implements the batching/caching "dataloader" pattern
// (https://github.com/graphql/dataloader) generically over Go generics, and
// wires up one instance per entity relationship used by the gateway's
// resolvers. A fresh set of loaders is created per HTTP request (see
// Middleware) so caching never leaks across requests.
package dataloaders

import (
	"context"
	"sync"
	"time"
)

// wait is how long a loader accumulates individual Load calls into a single
// key set before firing the batch function. It's intentionally tiny: just
// enough for goroutines started within the same "tick" of GraphQL field
// resolution to pile their keys onto the same batch.
const wait = 1 * time.Millisecond

// maxBatch caps how many keys go into a single downstream call. 0 means
// unlimited, which is fine for the mock services used in this project.
const maxBatch = 0

// BatchFunc fetches values for a set of keys in one round trip. It must
// return a result (value or error) for every key it was given.
type BatchFunc[K comparable, V any] func(ctx context.Context, keys []K) (map[K]V, map[K]error)

// Loader batches and caches calls to a BatchFunc across a single request.
// It is safe for concurrent use, which is what lets independent GraphQL
// field resolvers call Load concurrently and still land in one batch.
type Loader[K comparable, V any] struct {
	fetch BatchFunc[K, V]

	mu      sync.Mutex
	cache   map[K]V
	pending map[K][]chan result[V]
	timer   *time.Timer
}

type result[V any] struct {
	value V
	err   error
}

// NewLoader creates a Loader backed by fetch, scoped to a single request.
func NewLoader[K comparable, V any](fetch BatchFunc[K, V]) *Loader[K, V] {
	return &Loader[K, V]{
		fetch:   fetch,
		cache:   make(map[K]V),
		pending: make(map[K][]chan result[V]),
	}
}

// Load fetches the value for a single key, transparently batching it with
// any other Load calls made within the same short window.
func (l *Loader[K, V]) Load(ctx context.Context, key K) (V, error) {
	l.mu.Lock()
	if v, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return v, nil
	}

	ch := make(chan result[V], 1)
	l.pending[key] = append(l.pending[key], ch)

	if l.timer == nil {
		l.timer = time.AfterFunc(wait, func() { l.dispatch(ctx) })
	}
	l.mu.Unlock()

	select {
	case r := <-ch:
		return r.value, r.err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// LoadAll fetches values for multiple keys concurrently, still batching them
// (and anything else Loaded around the same time) into as few downstream
// calls as possible. The returned slice is ordered like keys; per-key errors
// come back positionally in the errs slice (nil where there was no error).
func (l *Loader[K, V]) LoadAll(ctx context.Context, keys []K) ([]V, []error) {
	values := make([]V, len(keys))
	errs := make([]error, len(keys))

	var wg sync.WaitGroup
	wg.Add(len(keys))
	for i, k := range keys {
		go func(i int, k K) {
			defer wg.Done()
			v, err := l.Load(ctx, k)
			values[i] = v
			errs[i] = err
		}(i, k)
	}
	wg.Wait()
	return values, errs
}

func (l *Loader[K, V]) dispatch(ctx context.Context) {
	l.mu.Lock()
	pending := l.pending
	l.pending = make(map[K][]chan result[V])
	l.timer = nil
	l.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	keys := make([]K, 0, len(pending))
	for k := range pending {
		keys = append(keys, k)
	}

	batches := [][]K{keys}
	if maxBatch > 0 && len(keys) > maxBatch {
		batches = chunk(keys, maxBatch)
	}

	for _, batch := range batches {
		values, errs := l.fetch(ctx, batch)

		l.mu.Lock()
		for _, k := range batch {
			r := result[V]{value: values[k], err: errs[k]}
			if r.err == nil {
				l.cache[k] = r.value
			}
			for _, ch := range pending[k] {
				ch <- r
				close(ch)
			}
		}
		l.mu.Unlock()
	}
}

func chunk[K any](keys []K, size int) [][]K {
	var out [][]K
	for size < len(keys) {
		keys, out = keys[size:], append(out, keys[:size:size])
	}
	return append(out, keys)
}
