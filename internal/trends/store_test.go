package trends

import (
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func TestStoreTopUsesSlidingWindow(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewStoreWithClock(5*time.Minute, time.Second, clock)

	store.Add("iphone", clock.now.Add(-4*time.Minute))
	store.Add("iphone", clock.now.Add(-2*time.Minute))
	store.Add("dress", clock.now.Add(-time.Minute))
	store.Add("old query", clock.now.Add(-6*time.Minute))

	top := store.Top(10)
	if len(top) != 2 {
		t.Fatalf("expected 2 items, got %d: %#v", len(top), top)
	}
	if top[0].Query != "iphone" || top[0].Count != 2 {
		t.Fatalf("unexpected first item: %#v", top[0])
	}

	clock.now = clock.now.Add(3*time.Minute + time.Second)
	top = store.Top(10)
	if len(top) != 1 || top[0].Query != "dress" {
		t.Fatalf("expected only dress to remain, got %#v", top)
	}
}

func TestStoreStopListFiltersTopAndIngest(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewStoreWithClock(5*time.Minute, time.Second, clock)

	store.Add("Spam", clock.now)
	store.Add("spam", clock.now)
	store.Add("sneakers", clock.now)
	store.AddStopWord(" spam ")
	store.Add("spam", clock.now)

	top := store.Top(10)
	if len(top) != 1 {
		t.Fatalf("expected 1 item, got %#v", top)
	}
	if top[0].Query != "sneakers" || top[0].Count != 1 {
		t.Fatalf("unexpected top: %#v", top)
	}

	store.DeleteStopWord("spam")
	top = store.Top(10)
	if len(top) != 2 || top[0].Query != "spam" || top[0].Count != 2 {
		t.Fatalf("expected historical spam counts after unblocking, got %#v", top)
	}
}

func TestStoreCapsQueryPerBucket(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)}
	store := NewStoreWithClock(5*time.Minute, time.Second, clock)
	store.SetMaxPerBucket(2)

	if !store.Add("iphone", clock.now) {
		t.Fatal("first event should be accepted")
	}
	if !store.Add("iphone", clock.now) {
		t.Fatal("second event should be accepted")
	}
	if store.Add("iphone", clock.now) {
		t.Fatal("third event in the same bucket should be rejected")
	}

	clock.now = clock.now.Add(time.Second)
	if !store.Add("iphone", clock.now) {
		t.Fatal("event in the next bucket should be accepted")
	}

	top := store.Top(1)
	if len(top) != 1 || top[0].Count != 3 {
		t.Fatalf("expected 3 accepted events, got %#v", top)
	}
}
