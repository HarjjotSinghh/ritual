package eval

import (
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

func TestBestMatchFindsTheEquivalentWorkflow(t *testing.T) {
	rep := &report.Report{Findings: []report.Finding{
		{Candidate: mine.Candidate{ID: "aaa", Title: "reconcile invoices", Keywords: []string{"invoice", "billing"}}},
		{Candidate: mine.Candidate{ID: "bbb", Title: "verify storefront theme", Keywords: []string{"storefront", "theme"}}},
	}}
	truth := Truth{Name: "storefront-verify", Tokens: []string{"storefront", "theme", "verify"}}

	rank, sim, id, _ := bestMatch(rep, truth, 0.45)
	if rank != 2 || id != "bbb" {
		t.Fatalf("rank = %d, id = %q", rank, id)
	}
	if sim <= 0 {
		t.Fatalf("similarity = %v", sim)
	}
}

func TestBestMatchReportsAMiss(t *testing.T) {
	rep := &report.Report{Findings: []report.Finding{
		{Candidate: mine.Candidate{ID: "aaa", Title: "reconcile invoices", Keywords: []string{"invoice", "billing"}}},
	}}
	truth := Truth{Name: "storefront-verify", Tokens: []string{"storefront", "theme", "verify"}}
	if rank, _, _, _ := bestMatch(rep, truth, 0.45); rank != 0 {
		t.Fatalf("rank = %d, want 0", rank)
	}
}

func TestSessionsBeforeIsExclusive(t *testing.T) {
	cutoff := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	sessions := []session.Session{
		{ID: "before", Start: cutoff.AddDate(0, 0, -1)},
		{ID: "after", Start: cutoff.AddDate(0, 0, 1)},
		{ID: "undated"},
	}
	got := sessionsBefore(sessions, cutoff)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want the earlier one and the undated one", len(got))
	}
}

func TestDefaultOptionsHideTheAnswerKey(t *testing.T) {
	// With the inventory enabled, every ground-truth workflow would classify
	// as "you already have this" and the benchmark would measure nothing.
	if !DefaultOptions().ReportOptions.SkipInventory {
		t.Fatal("the benchmark would see the skills it is being tested on")
	}
}
