// Package ui serves ritual's local dashboard.
//
// The dashboard is the answer to the obvious product question: why not put this
// on the web? Because the input is the operator's entire working history, and
// no amount of policy makes uploading that reasonable. So the browser
// experience runs here, against a report that never left the machine, bound to
// the loopback interface and gated by a token printed in the terminal.
package ui

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
	"github.com/HarjjotSinghh/ritual/internal/install"
	"github.com/HarjjotSinghh/ritual/internal/report"
)

//go:embed assets/*
var assets embed.FS

// Server holds one dashboard session.
type Server struct {
	report *report.Report
	token  string
	// allowWrite gates the install endpoint. A read-only dashboard is the
	// default because a page that can write to ~/.claude on a GET is a page
	// that can be made to do so by any other tab.
	allowWrite bool
	tmpl       *template.Template
}

// New builds a server for a report.
func New(rep *report.Report, allowWrite bool) (*Server, error) {
	tmpl, err := template.ParseFS(assets, "assets/index.html")
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &Server{report: rep, token: token, allowWrite: allowWrite, tmpl: tmpl}, nil
}

// Token is the value that must appear in the URL.
func (s *Server) Token() string { return s.token }

// Handler builds the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/report", s.guard(s.handleReport))
	mux.HandleFunc("/api/artifact", s.guard(s.handleArtifact))
	mux.HandleFunc("/api/install", s.guard(s.handleInstall))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))
	return securityHeaders(mux)
}

// Serve listens on the loopback interface and blocks. It returns the bound
// address through ready before serving so the caller can print a URL that is
// correct even when port 0 was requested.
func (s *Server) Serve(addr string, ready func(url string)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if ready != nil {
		ready(fmt.Sprintf("http://%s/?token=%s", ln.Addr().String(), s.token))
	}
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.Serve(ln)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !s.authorized(r) {
		http.Error(w, "missing or invalid token; open the URL ritual printed in your terminal", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Token      string
		AllowWrite bool
		Generated  string
	}{s.token, s.allowWrite, s.report.GeneratedAt.Format("2006-01-02 15:04 MST")}
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.report)
}

func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	f, ok := s.report.Find(id)
	if ok {
		writeJSON(w, http.StatusOK, artifact.Build(f))
		return
	}
	rule, ok := s.report.FindRule(id)
	if ok {
		writeJSON(w, http.StatusOK, artifact.BuildRule(rule))
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no finding with id " + id})
}

type installRequest struct {
	ID      string   `json:"id"`
	Targets []string `json:"targets"`
	DryRun  bool     `json:"dry_run"`
	Force   bool     `json:"force"`
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var req installRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !s.allowWrite && !req.DryRun {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "this dashboard is read-only; restart with `ritual ui --allow-install` to install from the browser",
		})
		return
	}

	var bundle artifact.Bundle
	if f, ok := s.report.Find(req.ID); ok {
		bundle = artifact.Build(f)
	} else if rule, ok := s.report.FindRule(req.ID); ok {
		bundle = artifact.BuildRule(rule)
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no finding with id " + req.ID})
		return
	}

	plans := make([]install.Plan, 0, len(req.Targets))
	applied := make([]string, 0, 4)
	for _, key := range req.Targets {
		t, ok := install.Lookup(key)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown target " + key})
			return
		}
		plan, err := install.PlanInstall(bundle, t, install.Options{Force: req.Force})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		plans = append(plans, plan)
		if req.DryRun {
			continue
		}
		changed, err := install.Apply(plan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		applied = append(applied, changed...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans, "applied": applied, "dry_run": req.DryRun})
}

// guard requires the token on every API route, including reads: the report
// holds prompts, and any page in the browser can issue a cross-origin GET.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}
		next(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) == 1
}

// securityHeaders keeps the page from reaching the network and keeps other
// origins from reading it.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src data:; font-src data:")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(body)
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
