package trends

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/leva/search-trends/internal/domain/trends"
)

func benchQueries(n int) []domain.SearchQuery {
	out := make([]domain.SearchQuery, n)
	for i := 0; i < n; i++ {
		q, err := domain.NewSearchQuery(fmt.Sprintf("query-%d", i))
		if err != nil {
			panic(err)
		}
		out[i] = q
	}
	return out
}

func BenchmarkStore_Add_SameQuery(b *testing.B) {
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	q, _ := domain.NewSearchQuery("iphone")
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Add(ctx, q, now)
	}
}

func BenchmarkStore_Add_UniqueQueries(b *testing.B) {
	const cardinality = 10_000
	queries := benchQueries(cardinality)
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Add(ctx, queries[i%cardinality], now)
	}
}

func BenchmarkStore_Add_Parallel(b *testing.B) {
	const cardinality = 1_000
	queries := benchQueries(cardinality)
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()
	var counter atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := counter.Add(1)
			_ = store.Add(ctx, queries[i%cardinality], now)
		}
	})
}

func BenchmarkStore_Top_Cached(b *testing.B) {
	const cardinality = 10_000
	queries := benchQueries(cardinality)
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < cardinality; i++ {
		_ = store.Add(ctx, queries[i], now)
	}
	_ = store.Top(ctx, 10)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Top(ctx, 10)
	}
}

func BenchmarkStore_Top_Dirty(b *testing.B) {
	const cardinality = 10_000
	queries := benchQueries(cardinality)
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < cardinality; i++ {
		_ = store.Add(ctx, queries[i], now)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Add(ctx, queries[i%cardinality], now)
		_ = store.Top(ctx, 10)
	}
}

func BenchmarkStore_ReadHeavyMix(b *testing.B) {
	const cardinality = 10_000
	queries := benchQueries(cardinality)
	store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < cardinality; i++ {
		_ = store.Add(ctx, queries[i], now)
	}
	_ = store.Top(ctx, 10)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			if r.Intn(100) < 5 {
				_ = store.Add(ctx, queries[r.Intn(cardinality)], now)
			} else {
				_ = store.Top(ctx, 10)
			}
		}
	})
}

func BenchmarkStore_Top_DifferentCardinalities(b *testing.B) {
	for _, cardinality := range []int{100, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("N=%d", cardinality), func(b *testing.B) {
			queries := benchQueries(cardinality)
			store := NewMemoryStore(5*time.Minute, time.Second, 1_000_000_000)
			ctx := context.Background()
			now := time.Now()
			for i := 0; i < cardinality; i++ {
				_ = store.Add(ctx, queries[i], now)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = store.Add(ctx, queries[i%cardinality], now)
				_ = store.Top(ctx, 10)
			}
		})
	}
}
