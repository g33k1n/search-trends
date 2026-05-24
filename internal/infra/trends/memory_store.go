package trends

import (
	"context"
	"sort"
	"sync"
	"time"

	apptrends "github.com/leva/search-trends/internal/application/trends"
	domain "github.com/leva/search-trends/internal/domain/trends"
)

var _ apptrends.TrendsStore = (*MemoryStore)(nil)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type bucket struct {
	start  time.Time
	counts map[string]int64
}

type MemoryStore struct {
	mu           sync.Mutex
	clock        Clock
	window       time.Duration
	resolution   time.Duration
	buckets      []bucket
	totals       map[string]int64
	stopList     map[string]struct{}
	snapshot     []domain.TrendEntry
	dirty        bool
	maxPerBucket int64
}

func NewMemoryStore(window, resolution time.Duration, maxPerBucket int64) *MemoryStore {
	if window <= 0 {
		window = 5 * time.Minute
	}
	if resolution <= 0 {
		resolution = time.Second
	}
	if maxPerBucket < 1 {
		maxPerBucket = 1000
	}
	bucketCount := int(window/resolution) + 1
	return &MemoryStore{
		clock:        realClock{},
		window:       window,
		resolution:   resolution,
		buckets:      make([]bucket, bucketCount),
		totals:       make(map[string]int64),
		stopList:     make(map[string]struct{}),
		dirty:        true,
		maxPerBucket: maxPerBucket,
	}
}

func NewMemoryStoreWithClock(window, resolution time.Duration, maxPerBucket int64, clock Clock) *MemoryStore {
	s := NewMemoryStore(window, resolution, maxPerBucket)
	if clock != nil {
		s.clock = clock
	}
	return s
}

func (s *MemoryStore) Window() time.Duration { return s.window }

func (s *MemoryStore) Add(_ context.Context, query domain.SearchQuery, at time.Time) error {
	if query.IsZero() {
		return domain.ErrEmptyQuery
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	value := query.String()
	if _, blocked := s.stopList[value]; blocked {
		return domain.ErrQueryBlocked
	}

	now := s.clock.Now()
	if at.IsZero() {
		at = now
	}
	if at.After(now.Add(s.resolution)) {
		at = now
	}
	if at.Before(now.Add(-s.window)) {
		return domain.ErrEventOutOfWindow
	}

	s.expireLocked(now)
	start := at.Truncate(s.resolution)
	idx := int(start.UnixNano()/s.resolution.Nanoseconds()) % len(s.buckets)
	if idx < 0 {
		idx += len(s.buckets)
	}

	b := &s.buckets[idx]
	if !b.start.Equal(start) {
		s.subtractBucketLocked(b)
		b.start = start
		b.counts = make(map[string]int64)
	}
	if s.maxPerBucket > 0 && b.counts[value] >= s.maxPerBucket {
		return domain.ErrRateLimited
	}

	b.counts[value]++
	s.totals[value]++
	s.dirty = true
	return nil
}

func (s *MemoryStore) Top(_ context.Context, limit int) []domain.TrendEntry {
	if limit <= 0 {
		limit = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.expireLocked(s.clock.Now())
	if s.dirty {
		s.rebuildSnapshotLocked()
	}

	if limit > len(s.snapshot) {
		limit = len(s.snapshot)
	}
	out := make([]domain.TrendEntry, limit)
	copy(out, s.snapshot[:limit])
	return out
}

func (s *MemoryStore) AddStop(_ context.Context, query domain.SearchQuery) error {
	if query.IsZero() {
		return domain.ErrEmptyQuery
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopList[query.String()] = struct{}{}
	s.dirty = true
	return nil
}

func (s *MemoryStore) RemoveStop(_ context.Context, query domain.SearchQuery) error {
	if query.IsZero() {
		return domain.ErrEmptyQuery
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.stopList[query.String()]; !ok {
		return domain.ErrStopWordNotFound
	}
	delete(s.stopList, query.String())
	s.dirty = true
	return nil
}

func (s *MemoryStore) ListStop(_ context.Context) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	words := make([]string, 0, len(s.stopList))
	for w := range s.stopList {
		words = append(words, w)
	}
	sort.Strings(words)
	return words
}

func (s *MemoryStore) expireLocked(now time.Time) {
	cutoff := now.Add(-s.window)
	for i := range s.buckets {
		if !s.buckets[i].start.IsZero() && s.buckets[i].start.Before(cutoff) {
			s.subtractBucketLocked(&s.buckets[i])
			s.buckets[i] = bucket{}
		}
	}
}

func (s *MemoryStore) subtractBucketLocked(b *bucket) {
	for q, count := range b.counts {
		s.totals[q] -= count
		if s.totals[q] <= 0 {
			delete(s.totals, q)
		}
	}
	if len(b.counts) > 0 {
		s.dirty = true
	}
}

func (s *MemoryStore) rebuildSnapshotLocked() {
	items := make([]domain.TrendEntry, 0, len(s.totals))
	for q, count := range s.totals {
		if count <= 0 {
			continue
		}
		if _, blocked := s.stopList[q]; blocked {
			continue
		}
		items = append(items, domain.TrendEntry{Query: q, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Query < items[j].Query
		}
		return items[i].Count > items[j].Count
	})
	s.snapshot = items
	s.dirty = false
}
