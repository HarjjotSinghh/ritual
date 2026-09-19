package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/report"
)

func server(t *testing.T, allowWrite bool) *Server {
	t.Helper()
	rep := &report.Report{
		Version: "test", GeneratedAt: time.Now().UTC(),
		Findings: []report.Finding{{
			Candidate: mine.Candidate{ID: "abc123", Title: "Verify", Slug: "verify"},
			Decision:  classify.Decision{Kind: classify.KindSkill},
		}},
	}
	s, err := New(rep, allowWrite)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAPIRequiresAToken(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/report", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: the report holds real prompts", rec.Code)
	}
}

func TestAPIAcceptsTheToken(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/report?token="+s.Token(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "abc123") {
		t.Fatal("the report was not served")
	}
}

func TestIndexRequiresAToken(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestReadOnlyServerRefusesToInstall(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"id":"abc123","targets":["claude"]}`)
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/install?token="+s.Token(), body))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestReadOnlyServerStillPlans(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"id":"abc123","targets":["claude"],"dry_run":true}`)
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/install?token="+s.Token(), body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a dry run", rec.Code)
	}
}

func TestSecurityHeadersArePresent(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/report?token="+s.Token(), nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("CSP = %q", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff is missing")
	}
}

func TestArtifactEndpointReturnsTheBundle(t *testing.T) {
	s := server(t, false)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/artifact?id=abc123&token="+s.Token(), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SKILL.md") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
