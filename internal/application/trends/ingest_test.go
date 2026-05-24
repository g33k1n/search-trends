package trends

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type stubStore struct {
	mu     sync.Mutex
	addErr error
	added  []domain.SearchQuery
}

func (s *stubStore) Add(_ context.Context, q domain.SearchQuery, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addErr != nil {
		return s.addErr
	}
	s.added = append(s.added, q)
	return nil
}
func (s *stubStore) Top(context.Context, int) []domain.TrendEntry         { return nil }
func (s *stubStore) AddStop(context.Context, domain.SearchQuery) error    { return nil }
func (s *stubStore) RemoveStop(context.Context, domain.SearchQuery) error { return nil }
func (s *stubStore) ListStop(context.Context) []string                    { return nil }
func (s *stubStore) Window() time.Duration                                { return 5 * time.Minute }

type stubMetrics struct {
	mu                                      sync.Mutex
	received, accepted, rejected, malformed int
}

func (m *stubMetrics) EventReceived()  { m.mu.Lock(); m.received++; m.mu.Unlock() }
func (m *stubMetrics) EventAccepted()  { m.mu.Lock(); m.accepted++; m.mu.Unlock() }
func (m *stubMetrics) EventRejected()  { m.mu.Lock(); m.rejected++; m.mu.Unlock() }
func (m *stubMetrics) EventMalformed() { m.mu.Lock(); m.malformed++; m.mu.Unlock() }

func TestIngestUseCaseRejectsEmptyQuery(t *testing.T) {
	store := &stubStore{}
	metrics := &stubMetrics{}
	uc := NewIngestEventUseCase(store, metrics)

	err := uc.Handle(context.Background(), IngestEventCommand{Query: "   "})
	if !errors.Is(err, domain.ErrEmptyQuery) {
		t.Fatalf("expected ErrEmptyQuery, got %v", err)
	}
	if metrics.rejected != 1 || metrics.accepted != 0 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if len(store.added) != 0 {
		t.Fatalf("store should not be called for empty query")
	}
}

func TestIngestUseCaseAccepts(t *testing.T) {
	store := &stubStore{}
	metrics := &stubMetrics{}
	uc := NewIngestEventUseCase(store, metrics)

	if err := uc.Handle(context.Background(), IngestEventCommand{Query: "  iPhone 15  "}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if metrics.accepted != 1 {
		t.Fatalf("expected accepted=1, got %d", metrics.accepted)
	}
	if len(store.added) != 1 || store.added[0].String() != "iphone 15" {
		t.Fatalf("unexpected store state: %+v", store.added)
	}
}

func TestIngestUseCasePropagatesStoreError(t *testing.T) {
	store := &stubStore{addErr: domain.ErrQueryBlocked}
	metrics := &stubMetrics{}
	uc := NewIngestEventUseCase(store, metrics)

	err := uc.Handle(context.Background(), IngestEventCommand{Query: "spam"})
	if !errors.Is(err, domain.ErrQueryBlocked) {
		t.Fatalf("expected ErrQueryBlocked, got %v", err)
	}
	if metrics.rejected != 1 || metrics.accepted != 0 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}
