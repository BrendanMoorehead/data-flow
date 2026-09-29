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
	store       *store.Store
	sources     source.Registry
	now         func() time.Time
	simAdminURL string
}

type ServerConfig struct {
	Store       *store.Store
	Sources     source.Registry
	Now         func() time.Time
	SimAdminURL string
}

func NewServer(config ServerConfig) *Server {
	return &Server{store: config.Store, sources: config.Sources, now: config.Now, simAdminURL: config.SimAdminURL}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveDashboard)
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
	quarantined, err := s.store.QuarantineCountsBySource(r.Context())
	if err != nil {
		httpserve.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	inputs := statusInputs{healths: healths, quarantined: quarantined, now: s.now()}
	httpserve.WriteJSON(w, http.StatusOK, statusResponseFrom(s.sources, inputs))
}
