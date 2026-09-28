package sim

import (
	"errors"
	"net/http"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
)

const (
	defaultScenarioDuration = 30 * time.Second
	maxScenarioDuration     = 10 * time.Minute
)

type scenarioActivation struct {
	Name        Scenario  `json:"name"`
	ActiveUntil time.Time `json:"active_until"`
}

func (s *Simulator) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /scenarios", s.serveScenarios)
	mux.HandleFunc("POST /scenarios/{name}", s.activateScenario)
	mux.HandleFunc("DELETE /scenarios/{name}", s.clearScenario)
	return mux
}

func (s *Simulator) serveScenarios(w http.ResponseWriter, r *http.Request) {
	httpserve.WriteJSON(w, http.StatusOK, map[string][]scenarioStatus{"scenarios": s.chaos.statuses()})
}

func (s *Simulator) activateScenario(w http.ResponseWriter, r *http.Request) {
	duration, err := scenarioDuration(r)
	if err != nil {
		httpserve.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	scenario := Scenario(r.PathValue("name"))
	activeUntil, err := s.chaos.activate(scenario, duration)
	if err != nil {
		writeScenarioError(w, err)
		return
	}
	httpserve.WriteJSON(w, http.StatusOK, scenarioActivation{Name: scenario, ActiveUntil: activeUntil})
}

func (s *Simulator) clearScenario(w http.ResponseWriter, r *http.Request) {
	if err := s.chaos.clear(Scenario(r.PathValue("name"))); err != nil {
		writeScenarioError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scenarioDuration(r *http.Request) (time.Duration, error) {
	text := r.URL.Query().Get("for")
	if text == "" {
		return defaultScenarioDuration, nil
	}
	duration, err := time.ParseDuration(text)
	if err != nil || duration <= 0 || duration > maxScenarioDuration {
		return 0, errors.New("for must be a positive duration up to 10m, e.g. 30s")
	}
	return duration, nil
}

func writeScenarioError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnknownScenario) {
		httpserve.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpserve.WriteError(w, http.StatusInternalServerError, err.Error())
}
