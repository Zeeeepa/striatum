package mutations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestTerminalJobStateWritesRouteThroughMarkJobTerminal protects the RFC 0112
// release hook. Direct job terminalization must route through markJobTerminal so
// explicit interrogation consumers can release windows and emit advisory
// required-target evidence.
func TestTerminalJobStateWritesRouteThroughMarkJobTerminal(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	literalTerminalJobUpdate := regexp.MustCompile(`(?s)UPDATE\s+striatumd\.jobs\b.*?SET\s+state\s*=\s*'(completed|failed|canceled|skipped|waiting_human)'`)
	dynamicStateJobUpdate := regexp.MustCompile(`(?s)UPDATE\s+striatumd\.jobs\b.*?SET\s+state\s*=\s*\$[0-9]+`)
	allowlisted := map[string]string{
		"cancelRunInTx": "bulk run cancellation closes remaining sessions as a structural run-finalization path",
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		file := fset.File(parsed.Pos())
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			body := string(src[file.Offset(fn.Pos()):file.Offset(fn.End())])
			terminalUpdate := literalTerminalJobUpdate.MatchString(body) ||
				(dynamicStateJobUpdate.MatchString(body) && strings.Contains(body, `"waiting_human"`))
			if !terminalUpdate {
				continue
			}
			if strings.Contains(body, "markJobTerminal(") {
				continue
			}
			if reason, ok := allowlisted[fn.Name.Name]; ok {
				t.Logf("allowlisted %s: %s", fn.Name.Name, reason)
				continue
			}
			t.Fatalf("%s contains a direct terminal striatumd.jobs update but does not call markJobTerminal", fn.Name.Name)
		}
	}
}
