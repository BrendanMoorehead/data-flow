package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardServesPageWithSimulatorAddress(t *testing.T) {
	server := NewServer(ServerConfig{SimAdminURL: "http://127.0.0.1:9100"})
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("GET / = %d %q, want 200 text/html", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(recorder.Body.String(), `"http://127.0.0.1:9100"`) {
		t.Error("dashboard does not contain the simulator admin address as a JavaScript string")
	}
}
