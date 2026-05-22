package trends

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type Item struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type bucket struct {
	start  time.Time
	counts map[string]int64
}

type Store struct {
	mu           sync.RWMutex
	clock        Clock
	window       time.Duration
	resolution   time.Duration
	buckets      []bucket
	totals       map[string]int64
	stopList     map[string]struct{}
	snapshot     []Item
	dirty        bool
	maxPerBucket int64
}

func NewStore(window, resolution time.Duration) *Store {
	if window <= 0 {
		window = 5 * time.Minute
	}
	if resolution <= 0 {
		resolution = time.Second
	}
	bucketCount := int(window/resolution) + 1
	return &Store{
		clock:        realClock{},
		window:       window,
		resolution:   resolution,
		buckets:      make([]bucket, bucketCount),
		totals:       make(map[string]int64),
		stopList:     make(map[string]struct{}),
		dirty:        true,
		maxPerBucket: 1000,
	}
}

func NewStoreWithClock(window, resolution time.Duration, clock Clock) *Store {
	s := NewStore(window, resolution)
	if clock != nil {
		s.clock = clock
	}
	return s
}

func (s *Store) Add(query string, at time.Time) bool {
	query = normalize(query)
	if query == "" {
		return false
	}
	if at.IsZero() {
		at = s.clock.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, blocked := s.stopList[query]; blocked {
		return false
	}

	now := s.clock.Now()
	if at.After(now.Add(s.resolution)) {
		at = now
	}
	if at.Before(now.Add(-s.window)) {
		return false
	}

	s.expireLocked(now)
	start := at.Truncate(s.resolution)
	idx := int(start.UnixNano()/s.resolution.Nanoseconds()) % len(s.buckets)
	if idx < 0 {
		idx = -idx
	}

	b := &s.buckets[idx]
	if !b.start.Equal(start) {
		s.subtractBucketLocked(b)
		b.start = start
		b.counts = make(map[string]int64)
	}
	if s.maxPerBucket > 0 && b.counts[query] >= s.maxPerBucket {
		return false
	}

	b.counts[query]++
	s.totals[query]++
	s.dirty = true
	return true
}

func (s *Store) SetMaxPerBucket(limit int64) {
	if limit < 1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxPerBucket = limit
}

func (s *Store) Top(limit int) []Item {
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
	out := make([]Item, limit)
	copy(out, s.snapshot[:limit])
	return out
}

func (s *Store) AddStopWord(query string) bool {
	query = normalize(query)
	if query == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopList[query] = struct{}{}
	s.dirty = true
	return true
}

func (s *Store) DeleteStopWord(query string) bool {
	query = normalize(query)
	if query == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.stopList[query]; !ok {
		return false
	}
	delete(s.stopList, query)
	s.dirty = true
	return true
}

func (s *Store) StopList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	words := make([]string, 0, len(s.stopList))
	for word := range s.stopList {
		words = append(words, word)
	}
	sort.Strings(words)
	return words
}

func (s *Store) Window() time.Duration {
	return s.window
}

func (s *Store) expireLocked(now time.Time) {
	cutoff := now.Add(-s.window)
	for i := range s.buckets {
		if !s.buckets[i].start.IsZero() && s.buckets[i].start.Before(cutoff) {
			s.subtractBucketLocked(&s.buckets[i])
			s.buckets[i] = bucket{}
		}
	}
}

func (s *Store) subtractBucketLocked(b *bucket) {
	for query, count := range b.counts {
		s.totals[query] -= count
		if s.totals[query] <= 0 {
			delete(s.totals, query)
		}
	}
	if len(b.counts) > 0 {
		s.dirty = true
	}
}

func (s *Store) rebuildSnapshotLocked() {
	items := make([]Item, 0, len(s.totals))
	for query, count := range s.totals {
		if count <= 0 {
			continue
		}
		if _, blocked := s.stopList[query]; blocked {
			continue
		}
		items = append(items, Item{Query: query, Count: count})
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

func normalize(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	return strings.Join(strings.Fields(query), " ")
}
