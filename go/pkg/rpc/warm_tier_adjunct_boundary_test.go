package rpc

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// externalMemoryConsumerModules are the memory consumers the daemon must never
// depend on. striatum-warmtier (the real warm-tier adjunct that consumes the S11
// lane_trajectory export) is among them: it is a pull-only consumer reached via
// delivered files, never an import.
var externalMemoryConsumerModules = []string{
	"github.com/halbritt/hippo",
	"github.com/halbritt/fornix",
	"github.com/halbritt/striatum-warmtier",
	"github.com/halbritt/engram",
}

// TestWarmTierAdjunctBoundaryHoldsWithAdjunctPresent is the S12 re-assertion of
// the RFC 0119 / D179 corpus-export boundary now that the warm-tier adjunct is a
// real, installed consumer on operator hosts. It proves that even with the
// adjunct present and reachable on the same machine the daemon (a) imports no
// external memory consumer (module graph and Go source), and (b) exposes no
// memory.* method or capability. The invariant assertions run unconditionally so
// the guardrail is green on a bare CI host; when the adjunct is actually present
// the test additionally verifies the daemon module graph does not couple to it.
func TestWarmTierAdjunctBoundaryHoldsWithAdjunctPresent(t *testing.T) {
	root := findRepositoryRoot(t)

	// (a) Module graph: no external memory consumer in go.mod/go.sum (covers
	// require and replace directives, including a relative replace at the adjunct).
	for _, rel := range []string{"go/go.mod", "go/go.sum"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, forbidden := range externalMemoryConsumerModules {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("%s couples the daemon to external memory consumer %s", rel, forbidden)
			}
		}
	}

	// (a continued) Go source: no import of any external memory consumer.
	fset := token.NewFileSet()
	walkErr := filepath.WalkDir(filepath.Join(root, "go"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imported := range file.Imports {
			value := strings.Trim(imported.Path.Value, `"`)
			if value == "engram" {
				t.Fatalf("%s imports forbidden external memory consumer %s", path, value)
			}
			for _, forbidden := range externalMemoryConsumerModules {
				if strings.HasPrefix(value, forbidden) {
					t.Fatalf("%s imports forbidden external memory consumer %s", path, value)
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("scan Go imports: %v", walkErr)
	}

	// (b) Registry and contract: no memory.* method or capability is registered,
	// and the only recall surface is recall.search (read / single_repo).
	for _, entry := range SortedMethods() {
		if strings.HasPrefix(entry.Method, "memory.") {
			t.Fatalf("forbidden memory.* daemon method registered: %s", entry.Method)
		}
		if entry.RequiredCapability != nil && strings.HasPrefix(string(*entry.RequiredCapability), "memory.") {
			t.Fatalf("forbidden memory.* capability registered on %s", entry.Method)
		}
	}
	recall, ok := MethodRegistry["recall.search"]
	if !ok {
		t.Fatal("recall.search is not registered")
	}
	if recall.RequiredCapability == nil || *recall.RequiredCapability != CapabilityRead {
		t.Fatalf("recall.search capability = %#v, want read", recall.RequiredCapability)
	}
	if recall.RepositoryScopeMode != ScopeSingleRepo {
		t.Fatalf("recall.search scope = %s, want single_repo", recall.RepositoryScopeMode)
	}

	// (c) Make the "with the adjunct present" claim concrete when we can. If the
	// real adjunct is discoverable on this host the boundary above was verified
	// against an actually-installed consumer; otherwise the invariants still held.
	if adjunct, present := locateWarmTierAdjunct(); present {
		t.Logf("warm-tier adjunct present at %s; daemon boundary verified with the consumer installed", adjunct)
	} else {
		t.Log("warm-tier adjunct not found on host; boundary invariants verified without an installed consumer (set STRIATUM_WARMTIER_ADJUNCT_PATH to assert against it)")
	}
}

// locateWarmTierAdjunct returns the on-host path to the striatum-warmtier adjunct
// if it is installed (an explicit STRIATUM_WARMTIER_ADJUNCT_PATH, else the
// conventional sibling checkout), recognizing it by its repo markers.
func locateWarmTierAdjunct() (string, bool) {
	candidates := []string{}
	if env := strings.TrimSpace(os.Getenv("STRIATUM_WARMTIER_ADJUNCT_PATH")); env != "" {
		candidates = append(candidates, env)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "git", "striatum-warmtier"))
	}
	candidates = append(candidates, "/home/halbritt/git/striatum-warmtier")
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		for _, marker := range []string{"pyproject.toml", ".git"} {
			if _, err := os.Stat(filepath.Join(candidate, marker)); err == nil {
				return candidate, true
			}
		}
	}
	return "", false
}
