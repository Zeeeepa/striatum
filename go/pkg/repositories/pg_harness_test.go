package repositories

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestAddRequiresDaemonRunnerAndRejectsTraversal(t *testing.T) {
	_, err := Service{}.Add(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": "."},
	})
	assertRPCErrorCode(t, err, "daemon_db_missing")

	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	traversal := repo + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(repo)
	_, err = Service{Runner: &resolveFakeRunner{}}.Add(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": traversal},
	})
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected parent traversal refusal, got %v", err)
	}
}
