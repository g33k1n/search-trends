package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/leva/search-trends/internal/ingest"
	"github.com/leva/search-trends/internal/trends"
)

type Server struct {
	store   *trends.Store
	metrics *ingest.Metrics
	mux     *http.ServeMux
	started time.Time
}

func NewServer(store *trends.Store, metrics *ingest.Metrics) *Server {
	s := &Server{
		store:   store,
		metrics: metrics,
		mux:     http.NewServeMux(),
		started: time.Now(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("GET /top", s.top)
	s.mux.HandleFunc("GET /stop-list", s.stopList)
	s.mux.HandleFunc("POST /stop-list", s.addStopWord)
	s.mux.HandleFunc("DELETE /stop-list/{query}", s.deleteStopWord)
	s.mux.HandleFunc("GET /metrics", s.prometheusMetrics)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) top(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be an integer from 1 to 1000")
			return
		}
		limit = parsed
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"window": s.store.Window().String(),
		"items":  s.store.Top(limit),
	})
}

func (s *Server) stopList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.StopList()})
}

func (s *Server) addStopWord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !s.store.AddStopWord(req.Query) {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (s *Server) deleteStopWord(w http.ResponseWriter, r *http.Request) {
	query := r.PathValue("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	if !s.store.DeleteStopWord(query) {
		writeError(w, http.StatusNotFound, "query is not in stop-list")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "search_trends_events_received_total %d\n", s.metrics.Received.Load())
	fmt.Fprintf(w, "search_trends_events_accepted_total %d\n", s.metrics.Accepted.Load())
	fmt.Fprintf(w, "search_trends_events_rejected_total %d\n", s.metrics.Rejected.Load())
	fmt.Fprintf(w, "search_trends_events_malformed_total %d\n", s.metrics.Malformed.Load())
	fmt.Fprintf(w, "search_trends_uptime_seconds %.0f\n", time.Since(s.started).Seconds())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}
