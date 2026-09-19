package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/report"
)

func TestConfigRoundTrip(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())

	cfg := DefaultConfig()
	cfg.Days = 45
	cfg.Author = "codex"
	cfg.Install = []string{"claude", "codex"}
	if _, err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, path, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Days != 45 || loaded.Author != "codex" || len(loaded.Install) != 2 {
		t.Fatalf("loaded = %+v from %s", loaded, path)
	}
}

func TestLoadConfigReturnsDefaultsWhenAbsent(t *testing.T) {
	t.Setenv(EnvHome, t.TempDir())
	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("a missing config must not be an error: %v", err)
	}
	if cfg.Days != DefaultConfig().Days {
		t.Fatalf("cfg = %+v, want defaults", cfg)
	}
}

func TestSaveReportIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvHome, dir)

	rep := &report.Report{Version: "test", GeneratedAt: time.Now().UTC()}
	path, err := SaveReport(rep)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not carry Unix permission bits, so the mode it reports says
	// nothing about who can read the file. The protection there comes from the
	// directory ACL that user profiles already have, which is out of this
	// package's hands and documented as such.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("report permissions = %o, want 600: reports hold real prompts", perm)
		}
	}

	loaded, _, err := LoadReport()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != "test" {
		t.Fatalf("round trip lost the report: %+v", loaded)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 25; i++ {
		name := filepath.Join(dir, time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02T150405Z")+".json")
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneReports(dir, 5); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Fatalf("kept %d reports, want 5", len(entries))
	}
}

func TestAgeReadsPlainly(t *testing.T) {
	if got := Age(time.Time{}); got != "unknown" {
		t.Fatalf("Age(zero) = %q", got)
	}
	if got := Age(time.Now().Add(-90 * time.Minute)); got != "1 hours ago" {
		t.Fatalf("Age = %q", got)
	}
}
