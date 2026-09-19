// Package version carries the build identity.
package version

import "runtime/debug"

// Version is the release version. It is overwritten at build time with
// -ldflags "-X github.com/HarjjotSinghh/ritual/internal/version.Version=v1.2.3"
// and falls back to the module's own build info for `go install` users.
var Version = "dev"

// Commit is the source revision, set the same way.
var Commit = ""

// Date is the build date in RFC3339, set the same way.
var Date = ""

// String renders the full identity for `ritual version`.
func String() string {
	v := Version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	out := "ritual " + v
	if Commit != "" {
		out += " (" + short(Commit) + ")"
	}
	if Date != "" {
		out += " built " + Date
	}
	return out
}

func short(commit string) string {
	if len(commit) > 10 {
		return commit[:10]
	}
	return commit
}
