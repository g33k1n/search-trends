package trends

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func mustQuery(t *testing.T, raw string) domain.SearchQuery {
	t.Helper()
	q, err := domain.NewSearchQuery(raw)
	if err != nil {
		t.Fatalf("NewSearchQuery(%q): %v", raw, err)
	}
	return q
}

func TestStoreTopUsesSlidingWindow(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewMemoryStoreWithClock(5*time.Minute, time.Second, 1000, clock)
	ctx := context.Background()

	mustAdd(t, store, ctx, "iphone", clock.now.Add(-4*time.Minute))
	mustAdd(t, store, ctx, "iphone", clock.now.Add(-2*time.Minute))
	mustAdd(t, store, ctx, "dress", clock.now.Add(-time.Minute))

	if err := store.Add(ctx, mustQuery(t, "old query"), clock.now.Add(-6*time.Minute)); !errors.Is(err, domain.ErrEventOutOfWindow) {
		t.Fatalf("expected ErrEventOutOfWindow, got %v", err)
	}

	top := store.Top(ctx, 10)
	if len(top) != 2 {
		t.Fatalf("expected 2 items, got %d: %#v", len(top), top)
	}
	if top[0].Query != "iphone" || top[0].Count != 2 {
		t.Fatalf("unexpected first item: %#v", top[0])
	}

	clock.now = clock.now.Add(3*time.Minute + time.Second)
	top = store.Top(ctx, 10)
	if len(top) != 1 || top[0].Query != "dress" {
		t.Fatalf("expected only dress to remain, got %#v", top)
	}
}

func TestStoreStopListFiltersTopAndIngest(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewMemoryStoreWithClock(5*time.Minute, time.Second, 1000, clock)
	ctx := context.Background()

	mustAdd(t, store, ctx, "Spam", clock.now)
	mustAdd(t, store, ctx, "spam", clock.now)
	mustAdd(t, store, ctx, "sneakers", clock.now)

	if err := store.AddStop(ctx, mustQuery(t, " spam ")); err != nil {
		t.Fatalf("AddStop: %v", err)
	}
	if err := store.Add(ctx, mustQuery(t, "spam"), clock.now); !errors.Is(err, domain.ErrQueryBlocked) {
		t.Fatalf("expected ErrQueryBlocked, got %v", err)
	}

	top := store.Top(ctx, 10)
	if len(top) != 1 || top[0].Query != "sneakers" || top[0].Count != 1 {
		t.Fatalf("unexpected top after stop-list: %#v", top)
	}

	if err := store.RemoveStop(ctx, mustQuery(t, "spam")); err != nil {
		t.Fatalf("RemoveStop: %v", err)
	}
	top = store.Top(ctx, 10)
	if len(top) != 2 || top[0].Query != "spam" || top[0].Count != 2 {
		t.Fatalf("expected historical spam counts after unblocking, got %#v", top)
	}
}

func TestStoreCapsQueryPerBucket(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewMemoryStoreWithClock(5*time.Minute, time.Second, 2, clock)
	ctx := context.Background()
	iphone := mustQuery(t, "iphone")

	if err := store.Add(ctx, iphone, clock.now); err != nil {
		t.Fatalf("first event: %v", err)
	}
	if err := store.Add(ctx, iphone, clock.now); err != nil {
		t.Fatalf("second event: %v", err)
	}
	if err := store.Add(ctx, iphone, clock.now); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	clock.now = clock.now.Add(time.Second)
	if err := store.Add(ctx, iphone, clock.now); err != nil {
		t.Fatalf("event in next bucket: %v", err)
	}

	top := store.Top(ctx, 1)
	if len(top) != 1 || top[0].Count != 3 {
		t.Fatalf("expected 3 accepted events, got %#v", top)
	}
}

func TestStoreParallelAddAndTopRaces(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewMemoryStoreWithClock(5*time.Minute, time.Second, 1_000_000, clock)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			_ = store.Add(ctx, mustQuery(t, "iphone"), clock.now)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			_ = store.Add(ctx, mustQuery(t, "sneakers"), clock.now)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 500; i++ {
			_ = store.Top(ctx, 5)
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = store.AddStop(ctx, mustQuery(t, "spam"))
			_ = store.RemoveStop(ctx, mustQuery(t, "spam"))
		}
		done <- struct{}{}
	}()
	for i := 0; i < 4; i++ {
		<-done
	}
}

func mustAdd(t *testing.T, store *MemoryStore, ctx context.Context, raw string, at time.Time) {
	t.Helper()
	if err := store.Add(ctx, mustQuery(t, raw), at); err != nil {
		t.Fatalf("Add(%q): %v", raw, err)
	}
}
