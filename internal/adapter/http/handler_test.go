package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	apptrends "github.com/g33k1n/search-trends/internal/application/trends"
	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type fakeStore struct {
	mu       sync.Mutex
	stopList map[string]struct{}
	top      []domain.TrendEntry
}

func newFakeStore(top []domain.TrendEntry) *fakeStore {
	return &fakeStore{stopList: map[string]struct{}{}, top: top}
}

func (s *fakeStore) Add(context.Context, domain.SearchQuery, time.Time) error { return nil }
func (s *fakeStore) Top(_ context.Context, limit int) []domain.TrendEntry {
	if limit > len(s.top) {
		limit = len(s.top)
	}
	return append([]domain.TrendEntry(nil), s.top[:limit]...)
}
func (s *fakeStore) AddStop(_ context.Context, q domain.SearchQuery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopList[q.String()] = struct{}{}
	return nil
}
func (s *fakeStore) RemoveStop(_ context.Context, q domain.SearchQuery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stopList[q.String()]; !ok {
		return domain.ErrStopWordNotFound
	}
	delete(s.stopList, q.String())
	return nil
}
func (s *fakeStore) ListStop(context.Context) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.stopList))
	for k := range s.stopList {
		out = append(out, k)
	}
	return out
}
func (s *fakeStore) Window() time.Duration { return 5 * time.Minute }

type noopMetrics struct{}

func (noopMetrics) EventReceived()  {}
func (noopMetrics) EventAccepted()  {}
func (noopMetrics) EventRejected()  {}
func (noopMetrics) EventMalformed() {}

func newTestServer(store apptrends.TrendsStore) http.Handler {
	queryUC := apptrends.NewQueryTopUseCase(store)
	stopListUC := apptrends.NewStopListUseCase(store)
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return NewServer(queryUC, stopListUC, metricsHandler).Handler()
}

func TestHealth(t *testing.T) {
	srv := newTestServer(newFakeStore(nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTopReturnsItems(t *testing.T) {
	store := newFakeStore([]domain.TrendEntry{
		{Query: "iphone", Count: 5},
		{Query: "sneakers", Count: 3},
	})
	srv := newTestServer(store)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/top?limit=2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Window string              `json:"window"`
		Items  []domain.TrendEntry `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 || body.Items[0].Query != "iphone" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestTopRejectsBadLimit(t *testing.T) {
	srv := newTestServer(newFakeStore(nil))
	for _, limit := range []string{"0", "1001", "abc", "-1"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/top?limit="+limit, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("limit=%q: expected 400, got %d", limit, rec.Code)
		}
	}
}

func TestStopListAddAndRemove(t *testing.T) {
	store := newFakeStore(nil)
	srv := newTestServer(store)

	addReq := httptest.NewRequest(http.MethodPost, "/stop-list", strings.NewReader(`{"query":"Spam"}`))
	addReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, addReq)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/stop-list/spam", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/stop-list/spam", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for absent stop word, got %d", rec.Code)
	}
}

func TestStopListEmptyBodyRejected(t *testing.T) {
	srv := newTestServer(newFakeStore(nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/stop-list", strings.NewReader(`{"query":"  "}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d", rec.Code)
	}
}
