package sim

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newCalmSimulator() *Simulator {
	return New(Options{Seed: 1, BackgroundProblems: false, Now: time.Now})
}

func statusOf(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestDraftKingsOutageReturnsServiceUnavailable(t *testing.T) {
	simulator := newCalmSimulator()
	if _, err := simulator.chaos.activate(ScenarioDraftKingsOutage, time.Minute); err != nil {
		t.Fatalf("activate: %v", err)
	}

	response := statusOf(t, simulator.DraftKingsHandler(), http.MethodGet, "/api/events?league=NBA")

	if response.Code != http.StatusServiceUnavailable {
		t.Errorf("status during outage = %d, want 503", response.Code)
	}
}

func TestAggregatorRateLimitAllowsOneRequestThenSendsRetryAfter(t *testing.T) {
	simulator := newCalmSimulator()
	simulator.chaos.activate(ScenarioAggregatorRateLimit, time.Minute)
	handler := simulator.AggregatorHandler()

	first := statusOf(t, handler, http.MethodGet, "/v1/odds?sport=basketball_nba")
	second := statusOf(t, handler, http.MethodGet, "/v1/odds?sport=basketball_nba")

	if first.Code != http.StatusOK || second.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = %d then %d, want 200 then 429", first.Code, second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("429 response has no Retry-After header")
	}
}

func TestAdminActivatesAndClearsScenarios(t *testing.T) {
	simulator := newCalmSimulator()
	admin := simulator.AdminHandler()

	activated := statusOf(t, admin, http.MethodPost, "/scenarios/draftkings-slow?for=10s")
	activeAfterPost := simulator.chaos.isActive(ScenarioDraftKingsSlow)
	cleared := statusOf(t, admin, http.MethodDelete, "/scenarios/draftkings-slow")

	if activated.Code != http.StatusOK || !activeAfterPost {
		t.Errorf("activate status = %d, active = %v; want 200 and active", activated.Code, activeAfterPost)
	}
	if cleared.Code != http.StatusNoContent || simulator.chaos.isActive(ScenarioDraftKingsSlow) {
		t.Errorf("clear status = %d, still active = %v; want 204 and inactive", cleared.Code, simulator.chaos.isActive(ScenarioDraftKingsSlow))
	}
}

func TestAdminRejectsUnknownScenario(t *testing.T) {
	response := statusOf(t, newCalmSimulator().AdminHandler(), http.MethodPost, "/scenarios/meteor-strike")

	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
}

func TestQuoteAsOfReturnsLatestQuoteNotAfterTheMoment(t *testing.T) {
	start := time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	view := gameView{quoteHistory: map[string][]moneylineQuote{bookFanDuel: {
		{home: -110, updatedAt: start},
		{home: -120, updatedAt: start.Add(10 * time.Second)},
		{home: -130, updatedAt: start.Add(20 * time.Second)},
	}}}

	if got := view.quoteAsOf(bookFanDuel, start.Add(15*time.Second)).home; got != -120 {
		t.Errorf("quote as of +15s = %v, want -120", got)
	}
}

func TestAdminAllowsDashboardToClearScenarios(t *testing.T) {
	simulator := New(Options{Seed: 1, Now: time.Now, DashboardOrigin: "http://127.0.0.1:8080"})
	request := httptest.NewRequest(http.MethodOptions, "/scenarios/draftkings-slow", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	recorder := httptest.NewRecorder()

	simulator.AdminHandler().ServeHTTP(recorder, request)

	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8080" {
		t.Errorf("preflight allow-origin = %q, want the dashboard origin", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAdminDoesNotAllowOtherOrigins(t *testing.T) {
	simulator := New(Options{Seed: 1, Now: time.Now, DashboardOrigin: "http://127.0.0.1:8080"})
	request := httptest.NewRequest(http.MethodGet, "/scenarios", nil)
	request.Header.Set("Origin", "http://evil.example")
	recorder := httptest.NewRecorder()

	simulator.AdminHandler().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin for a foreign origin = %q, want none", got)
	}
}
