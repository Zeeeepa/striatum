package rpc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// RFC 0111 P3: the error-code catalog is a closed contract, guard-reconciled
// in both directions against the Go source under go/ (mirroring the spirit of
// registry_contract_test.go and the seat-tier guards):
//
//   - every error-code literal a handler can return must have a catalog entry
//     (no uncataloged codes), and
//   - every catalog entry must correspond to at least one source literal
//     (no orphaned catalog entries surviving code removal).
//
// The scan covers the literal shapes that provably become agent-visible error
// codes:
//
//   - NewError("<code>" / rpc.NewError("<code>"  — the rpc.Error constructor;
//   - Code: "<code>"                             — rpc.Error / client-side
//     error struct literals (pkg/cli/rpcclient, the MCP local-request guard);
//   - DenialReason: "<code>" / DenialReason = "<code>" — capability denial
//     reasons, which RequireAllowed converts verbatim into rpc.Error codes
//     (go/pkg/rpc/capability.go).
//
// Test files are excluded (tests invent throwaway codes), as is the catalog
// itself (its own Code: literals must not satisfy reverse reconciliation).
var errorCodeLiteralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`NewError\(\s*"([a-z0-9_]+)"`),
	regexp.MustCompile(`\bCode:\s*"([a-z0-9_]+)"`),
	regexp.MustCompile(`DenialReason\s*[:=]\s*"([a-z0-9_]+)"`),
}

func TestErrorCatalogHasNoDuplicateOrEmptyEntries(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range ErrorCatalog {
		if entry.Code == "" {
			t.Fatalf("catalog contains an entry with an empty code")
		}
		if entry.Meaning == "" {
			t.Errorf("catalog entry %s has no meaning", entry.Code)
		}
		if seen[entry.Code] {
			t.Errorf("catalog contains duplicate code %s", entry.Code)
		}
		seen[entry.Code] = true
	}
}

func TestErrorCatalogIsClosedAgainstSource(t *testing.T) {
	sourceCodes := scanErrorCodeLiterals(t)
	if len(sourceCodes) == 0 {
		t.Fatalf("source scan found no error-code literals; scan patterns are broken")
	}

	cataloged := map[string]bool{}
	for _, entry := range ErrorCatalog {
		cataloged[entry.Code] = true
	}

	uncataloged := make([]string, 0)
	for code := range sourceCodes {
		if !cataloged[code] {
			uncataloged = append(uncataloged, code)
		}
	}
	sort.Strings(uncataloged)
	if len(uncataloged) > 0 {
		t.Errorf("error codes emitted in source but absent from ErrorCatalog (add entries in error_catalog.go): %v", uncataloged)
	}

	orphaned := make([]string, 0)
	for code := range cataloged {
		if _, exists := sourceCodes[code]; !exists {
			orphaned = append(orphaned, code)
		}
	}
	sort.Strings(orphaned)
	if len(orphaned) > 0 {
		t.Errorf("ErrorCatalog entries match no source literal (remove stale entries from error_catalog.go): %v", orphaned)
	}
}

// TestErrorCatalogLookupAgreesWithEntries pins the lookup helpers the P2
// default-fill depends on to the catalog content.
func TestErrorCatalogLookupAgreesWithEntries(t *testing.T) {
	for _, entry := range ErrorCatalog {
		got, ok := LookupErrorCode(entry.Code)
		if !ok {
			t.Fatalf("LookupErrorCode(%q) not found", entry.Code)
		}
		if got != entry {
			t.Fatalf("LookupErrorCode(%q) = %#v, want %#v", entry.Code, got, entry)
		}
		if DefaultSuggestion(entry.Code) != entry.Suggestion {
			t.Fatalf("DefaultSuggestion(%q) drifts from catalog entry", entry.Code)
		}
	}
	if _, ok := LookupErrorCode("definitely_not_a_real_code"); ok {
		t.Fatalf("LookupErrorCode must miss on unknown codes")
	}
	if DefaultSuggestion("definitely_not_a_real_code") != "" {
		t.Fatalf("DefaultSuggestion must be empty for unknown codes")
	}
}

// TestErrorCatalogMatrixDocCarriesEveryCode is the doc<->code drift guard for
// the "Error code catalog" section of command-authority-matrix.md: every
// cataloged code must appear as a `code` table row in the doc.
func TestErrorCatalogMatrixDocCarriesEveryCode(t *testing.T) {
	docPath := filepath.Join(findRepositoryRoot(t), "docs", "reference", "command-authority-matrix.md")
	payload, err := os.ReadFile(docPath)
	if os.IsNotExist(err) {
		t.Skip("docs/reference/command-authority-matrix.md is not present in this checkout")
	}
	if err != nil {
		t.Fatalf("read authority matrix doc: %v", err)
	}
	doc := string(payload)
	missing := make([]string, 0)
	for _, entry := range ErrorCatalog {
		if !strings.Contains(doc, "| `"+entry.Code+"`") {
			missing = append(missing, entry.Code)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("command-authority-matrix.md error-code catalog section is missing rows for: %v", missing)
	}
}

func scanErrorCodeLiterals(t *testing.T) map[string][]string {
	t.Helper()
	root := filepath.Join(findRepositoryRoot(t), "go")
	codes := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if name == "error_catalog.go" {
			return nil
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(payload)
		for _, pattern := range errorCodeLiteralPatterns {
			for _, match := range pattern.FindAllStringSubmatch(content, -1) {
				code := match[1]
				codes[code] = append(codes[code], path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan go/ source for error-code literals: %v", err)
	}
	return codes
}
