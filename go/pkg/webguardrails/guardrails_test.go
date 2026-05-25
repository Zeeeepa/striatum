package webguardrails

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoWebCutoverDoesNotImportPythonWebService(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"go/cmd/striatumd",
		"go/pkg/webservice",
		"go/pkg/webassets",
		"go/pkg/websse",
		"go/pkg/webtest",
	} {
		dir := filepath.Join(root, rel)
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			text := string(body)
			for _, forbidden := range []string{
				"src/striatum/service.py",
				"striatum.service",
				"striatum.web",
				"sqlite3",
				".striatum/retired-local-state",
				"terminal output",
				"transcript",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s contains retired web/service authority marker %q", path, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetiredRouteNamesRemainDocumentedInGuardScript(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "scripts/guard_rfc0078_web_retirement.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, route := range []string{"/dogfood", "/chat"} {
		if !strings.Contains(text, route) {
			t.Fatalf("guard script does not name retired route %s", route)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
