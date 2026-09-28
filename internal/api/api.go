package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

type Server struct {
	store   *store.Store
	sources source.Registry
	now     func() time.Time
}

func NewServer(store *store.Store, sources source.Registry, now func() time.Time) *Server {
	return &Server{store: store, sources: sources, now: now}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", s.serveEvents)
	mux.HandleFunc("GET /events/{eventID}/odds", s.serveEventOdds)
	mux.HandleFunc("GET /status", s.serveStatus)
	return mux
}

func (s *Server) serveEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.ListEvents(r.Context())
	if err != nil {
		httpserve.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpserve.WriteJSON(w, http.StatusOK, eventsResponseFrom(events))
}

func (s *Server) serveEventOdds(w http.ResponseWriter, r *http.Request) {
	eventID, err := strconv.ParseInt(r.PathValue("eventID"), 10, 64)
	if err != nil {
		httpserve.WriteError(w, http.StatusBadRequest, "event id must be an integer")
		return
	}
	prices, err := s.store.ResolvedPricesForEvent(r.Context(), canonical.EventID(eventID))
	if err != nil {
		httpserve.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpserve.WriteJSON(w, http.StatusOK, s.oddsResponseFrom(eventID, prices))
}

func (s *Server) serveStatus(w http.ResponseWriter, r *http.Request) {
	healths, err := s.store.ListSliceHealth(r.Context())
	if err != nil {
		httpserve.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpserve.WriteJSON(w, http.StatusOK, statusResponseFrom(s.sources, healths, s.now()))
}
