package rpc

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoExternalMemoryConsumerImports(t *testing.T) {
	root := findRepositoryRoot(t)
	for _, rel := range []string{"go/go.mod"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, forbidden := range []string{
			"github.com/halbritt/hippo",
			"github.com/halbritt/fornix",
			"github.com/halbritt/striatum-warmtier",
			"github.com/halbritt/engram",
		} {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("%s contains forbidden external memory dependency %s", rel, forbidden)
			}
		}
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join(root, "go"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			value := strings.Trim(imported.Path.Value, `"`)
			if value == "engram" ||
				strings.HasPrefix(value, "github.com/halbritt/hippo") ||
				strings.HasPrefix(value, "github.com/halbritt/fornix") ||
				strings.HasPrefix(value, "github.com/halbritt/striatum-warmtier") ||
				strings.HasPrefix(value, "github.com/halbritt/engram") {
				t.Fatalf("%s imports forbidden external memory consumer %s", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan Go imports: %v", err)
	}
}
