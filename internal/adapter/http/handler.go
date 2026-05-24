package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	apptrends "github.com/g33k1n/search-trends/internal/application/trends"
	domain "github.com/g33k1n/search-trends/internal/domain/trends"
)

type Server struct {
	queryTop *apptrends.QueryTopUseCase
	stopList *apptrends.StopListUseCase
	mux      *http.ServeMux
}

func NewServer(
	queryTop *apptrends.QueryTopUseCase,
	stopList *apptrends.StopListUseCase,
	metricsHandler http.Handler,
) *Server {
	s := &Server{
		queryTop: queryTop,
		stopList: stopList,
		mux:      http.NewServeMux(),
	}
	s.routes(metricsHandler)
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes(metricsHandler http.Handler) {
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("HEAD /health", s.health)
	s.mux.HandleFunc("GET /top", s.top)
	s.mux.HandleFunc("GET /stop-list", s.listStop)
	s.mux.HandleFunc("POST /stop-list", s.addStop)
	s.mux.HandleFunc("DELETE /stop-list/{query}", s.removeStop)
	s.mux.Handle("GET /metrics", metricsHandler)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
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

	result := s.queryTop.Handle(r.Context(), apptrends.TopQuery{Limit: limit})
	writeJSON(w, http.StatusOK, map[string]any{
		"window": result.Window.String(),
		"items":  result.Items,
	})
}

func (s *Server) listStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.stopList.List(r.Context())})
}

func (s *Server) addStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.stopList.Add(r.Context(), req.Query); err != nil {
		if errors.Is(err, domain.ErrEmptyQuery) {
			writeError(w, http.StatusBadRequest, "query is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add stop word")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (s *Server) removeStop(w http.ResponseWriter, r *http.Request) {
	query := r.PathValue("query")
	if err := s.stopList.Remove(r.Context(), query); err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyQuery):
			writeError(w, http.StatusBadRequest, "query is required")
		case errors.Is(err, domain.ErrStopWordNotFound):
			writeError(w, http.StatusNotFound, "query is not in stop-list")
		default:
			writeError(w, http.StatusInternalServerError, "failed to remove stop word")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}
