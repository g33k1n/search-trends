package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSinkCountersExposition(t *testing.T) {
	sink := NewSink()
	sink.EventReceived()
	sink.EventReceived()
	sink.EventAccepted()
	sink.EventRejected()
	sink.EventMalformed()

	rec := httptest.NewRecorder()
	sink.HTTPHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()

	mustContain := []string{
		"# TYPE search_trends_events_received_total counter",
		"search_trends_events_received_total 2",
		"# TYPE search_trends_events_accepted_total counter",
		"search_trends_events_accepted_total 1",
		"search_trends_events_rejected_total 1",
		"search_trends_events_malformed_total 1",
		"# TYPE search_trends_uptime_seconds gauge",
	}
	for _, want := range mustContain {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q\nbody:\n%s", want, body)
		}
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", got)
	}
}

func TestSinkConcurrentIncrement(t *testing.T) {
	sink := NewSink()
	const goroutines = 50
	const perGoroutine = 1_000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				sink.EventReceived()
				sink.EventAccepted()
			}
		}()
	}
	wg.Wait()

	if got := sink.received.Load(); got != goroutines*perGoroutine {
		t.Errorf("received = %d, want %d", got, goroutines*perGoroutine)
	}
	if got := sink.accepted.Load(); got != goroutines*perGoroutine {
		t.Errorf("accepted = %d, want %d", got, goroutines*perGoroutine)
	}
}
