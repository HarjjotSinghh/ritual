// Package store owns ritual's own directory: configuration, saved reports, and
// generated artifacts.
//
// Everything here stays on the machine that produced it. There is no upload, no
// account, and no telemetry — not as a feature to be turned off, but because
// the data being processed is the operator's entire working history, and a tool
// that moved it anywhere would be indefensible regardless of what it promised.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/HarjjotSinghh/ritual/internal/report"
)

// EnvHome overrides the ritual directory, mainly for tests and for operators
// who keep dotfiles somewhere deliberate.
const EnvHome = "RITUAL_HOME"

// Dir returns ritual's directory, creating it when missing.
func Dir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(EnvHome)); custom != "" {
		return custom, os.MkdirAll(custom, 0o700)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ritual")
	return dir, os.MkdirAll(dir, 0o700)
}

// Config is the persisted configuration.
type Config struct {
	// Agents restricts scans to these catalog keys. Empty means all installed.
	Agents []string `toml:"agents"`
	// Days is the default lookback for a scan.
	Days int `toml:"days"`
	// MinScore is the total below which a candidate is classified as ignore.
	MinScore float64 `toml:"min_score"`
	// MinOccurrences is how many runs a workflow needs to be proposed.
	MinOccurrences int `toml:"min_occurrences"`
	// MinSessions is how many distinct sessions a workflow must span.
	MinSessions int `toml:"min_sessions"`
	// Threshold is the clustering similarity cutoff.
	Threshold float64 `toml:"threshold"`
	// KeepEmails disables email redaction for a local-only run.
	KeepEmails bool `toml:"keep_emails"`
	// Author names the local agent CLI used to write artifact prose, or
	// "none" to always use the deterministic template.
	Author string `toml:"author"`
	// Install lists the harnesses `ritual install` writes to by default.
	Install []string `toml:"install"`
	// Weights overrides the scoring blend.
	Weights map[string]float64 `toml:"weights"`
}

// DefaultConfig is what a fresh install behaves like.
func DefaultConfig() Config {
	return Config{
		Days:           90,
		MinScore:       28,
		MinOccurrences: 3,
		MinSessions:    2,
		Threshold:      0.42,
		Author:         "auto",
		Install:        []string{"claude"},
	}
}

// LoadConfig reads config.toml, returning defaults when it does not exist.
func LoadConfig() (Config, string, error) {
	cfg := DefaultConfig()
	dir, err := Dir()
	if err != nil {
		return cfg, "", err
	}
	path := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, path, nil
	}
	if err != nil {
		return cfg, path, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, path, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, path, nil
}

// SaveConfig writes config.toml.
func SaveConfig(cfg Config) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.toml")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return path, err
	}
	defer f.Close()
	if _, err := f.WriteString("# ritual configuration. See `ritual config --help`.\n"); err != nil {
		return path, err
	}
	return path, toml.NewEncoder(f).Encode(cfg)
}

// ReportsDir is where saved reports live.
func ReportsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	reports := filepath.Join(dir, "reports")
	return reports, os.MkdirAll(reports, 0o700)
}

// SaveReport writes a report as the latest and as a timestamped copy, and
// returns the path of the latest.
//
// Reports contain real prompts, so they are written with owner-only permissions
// and the directory is not world-readable.
func SaveReport(rep *report.Report) (string, error) {
	dir, err := ReportsDir()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	stamped := filepath.Join(dir, rep.GeneratedAt.Format("2006-01-02T150405Z")+".json")
	if err := os.WriteFile(stamped, data, 0o600); err != nil {
		return "", err
	}
	latest := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(latest, data, 0o600); err != nil {
		return "", err
	}
	if err := pruneReports(dir, 20); err != nil {
		return latest, err
	}
	return latest, nil
}

// LoadReport reads the most recent saved report.
func LoadReport() (*report.Report, string, error) {
	dir, err := ReportsDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "latest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, path, fmt.Errorf("parse %s: %w", path, err)
	}
	return &rep, path, nil
}

// pruneReports keeps the newest n timestamped reports. History is useful for
// seeing a workflow appear over time; unbounded history is a slow leak of the
// operator's prompts onto disk.
func pruneReports(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	stamped := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || e.Name() == "latest.json" || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		stamped = append(stamped, e.Name())
	}
	if len(stamped) <= keep {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(stamped)))
	for _, name := range stamped[keep:] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// OutDir is where generated artifacts are written before installation.
func OutDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "out")
	return out, os.MkdirAll(out, 0o700)
}

// Age renders how old a report is, so a stale one announces itself.
func Age(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
