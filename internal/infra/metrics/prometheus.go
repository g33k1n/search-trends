package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	apptrends "github.com/leva/search-trends/internal/application/trends"
)

var _ apptrends.MetricsSink = (*Sink)(nil)

type Sink struct {
	received  atomic.Uint64
	accepted  atomic.Uint64
	rejected  atomic.Uint64
	malformed atomic.Uint64
	started   time.Time
}

func NewSink() *Sink {
	return &Sink{started: time.Now()}
}

func (s *Sink) EventReceived()  { s.received.Add(1) }
func (s *Sink) EventAccepted()  { s.accepted.Add(1) }
func (s *Sink) EventRejected()  { s.rejected.Add(1) }
func (s *Sink) EventMalformed() { s.malformed.Add(1) }

func (s *Sink) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		s.write(w)
	})
}

func (s *Sink) write(w io.Writer) {
	counter := func(name, help string, value uint64) {
		fmt.Fprintf(w, "# HELP %s %s\n", name, help)
		fmt.Fprintf(w, "# TYPE %s counter\n", name)
		fmt.Fprintf(w, "%s %d\n", name, value)
	}
	counter("search_trends_events_received_total", "Search events received from the broker.", s.received.Load())
	counter("search_trends_events_accepted_total", "Search events accepted into the sliding window.", s.accepted.Load())
	counter("search_trends_events_rejected_total", "Search events rejected by stop-list, rate limit, or out-of-window.", s.rejected.Load())
	counter("search_trends_events_malformed_total", "Search events that failed to deserialize.", s.malformed.Load())

	fmt.Fprintf(w, "# HELP search_trends_uptime_seconds Service uptime in seconds.\n")
	fmt.Fprintf(w, "# TYPE search_trends_uptime_seconds gauge\n")
	fmt.Fprintf(w, "search_trends_uptime_seconds %.0f\n", time.Since(s.started).Seconds())
}
