package dataloaders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLoader_Batching verifies that N concurrent Load calls for distinct
// keys, issued within the same tick, collapse into a single batch-fetch
// call — the core promise of the dataloader pattern.
func TestLoader_Batching(t *testing.T) {
	var batchCalls atomic.Int32
	var keysSeen atomic.Int32

	loader := NewLoader(func(ctx context.Context, keys []string) (map[string]int, map[string]error) {
		batchCalls.Add(1)
		keysSeen.Add(int32(len(keys)))
		values := make(map[string]int, len(keys))
		for _, k := range keys {
			values[k] = len(k)
		}
		return values, nil
	})

	ctx := context.Background()
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", i)
			v, err := loader.Load(ctx, key)
			if err != nil {
				t.Errorf("unexpected error for key %s: %v", key, err)
				return
			}
			if v != len(key) {
				t.Errorf("key %s: got %d, want %d", key, v, len(key))
			}
		}(i)
	}
	wg.Wait()

	if got := batchCalls.Load(); got != 1 {
		t.Errorf("expected exactly 1 batch call, got %d", got)
	}
	if got := keysSeen.Load(); got != n {
		t.Errorf("expected %d keys seen across batch calls, got %d", n, got)
	}
}

// TestLoader_Caching verifies that a key loaded once within a request is
// served from cache on subsequent loads, without hitting the batch function
// again.
func TestLoader_Caching(t *testing.T) {
	var batchCalls atomic.Int32

	loader := NewLoader(func(ctx context.Context, keys []string) (map[string]string, map[string]error) {
		batchCalls.Add(1)
		values := make(map[string]string, len(keys))
		for _, k := range keys {
			values[k] = "value-" + k
		}
		return values, nil
	})

	ctx := context.Background()
	if _, err := loader.Load(ctx, "a"); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if _, err := loader.Load(ctx, "a"); err != nil {
		t.Fatalf("second load: %v", err)
	}
	v, err := loader.Load(ctx, "a")
	if err != nil {
		t.Fatalf("third load: %v", err)
	}
	if v != "value-a" {
		t.Errorf("got %q, want %q", v, "value-a")
	}

	if got := batchCalls.Load(); got != 1 {
		t.Errorf("expected exactly 1 batch call after repeated loads of the same key, got %d", got)
	}
}

// TestLoader_PerKeyError verifies that one key's failure is reported only
// for that key and doesn't prevent other keys in the same batch from
// resolving successfully.
func TestLoader_PerKeyError(t *testing.T) {
	loader := NewLoader(func(ctx context.Context, keys []string) (map[string]string, map[string]error) {
		values := make(map[string]string, len(keys))
		errs := make(map[string]error, len(keys))
		for _, k := range keys {
			if k == "missing" {
				errs[k] = errors.New("not found")
				continue
			}
			values[k] = "ok-" + k
		}
		return values, errs
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(2)

	var goodVal string
	var goodErr, badErr error
	go func() {
		defer wg.Done()
		goodVal, goodErr = loader.Load(ctx, "good")
	}()
	go func() {
		defer wg.Done()
		_, badErr = loader.Load(ctx, "missing")
	}()
	wg.Wait()

	if goodErr != nil {
		t.Errorf("expected no error for 'good' key, got %v", goodErr)
	}
	if goodVal != "ok-good" {
		t.Errorf("got %q, want %q", goodVal, "ok-good")
	}
	if badErr == nil {
		t.Error("expected an error for 'missing' key, got nil")
	}
}

// TestLoader_ContextCancellation verifies that Load respects context
// cancellation instead of blocking forever when the batch function never
// completes.
func TestLoader_ContextCancellation(t *testing.T) {
	block := make(chan struct{})
	loader := NewLoader(func(ctx context.Context, keys []string) (map[string]int, map[string]error) {
		<-block // never returns until the test unblocks it
		return nil, nil
	})
	defer close(block)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := loader.Load(ctx, "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestLoader_LoadAll verifies the concurrent multi-key helper returns
// values positionally aligned with the input keys.
func TestLoader_LoadAll(t *testing.T) {
	loader := NewLoader(func(ctx context.Context, keys []string) (map[string]int, map[string]error) {
		values := make(map[string]int, len(keys))
		for _, k := range keys {
			values[k] = len(k)
		}
		return values, nil
	})

	ctx := context.Background()
	keys := []string{"a", "bb", "ccc"}
	values, errs := loader.LoadAll(ctx, keys)

	for i, k := range keys {
		if errs[i] != nil {
			t.Errorf("key %s: unexpected error %v", k, errs[i])
		}
		if values[i] != len(k) {
			t.Errorf("key %s: got %d, want %d", k, values[i], len(k))
		}
	}
}
