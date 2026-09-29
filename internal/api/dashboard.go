package api

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed dashboard.html
var dashboardHTML string

var dashboardTemplate = template.Must(template.New("dashboard").Parse(dashboardHTML))

type dashboardData struct {
	SimAdminURL string
}

func (s *Server) serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(w, dashboardData{SimAdminURL: s.simAdminURL}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
