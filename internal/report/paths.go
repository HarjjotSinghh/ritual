package report

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome reverses the redaction applied at ingest so the inventory scan can
// actually reach a project directory. Redaction protects what leaves the
// machine; the local filesystem still needs real paths.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "${HOME}") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, strings.TrimPrefix(p, "${HOME}"))
}
