package reads

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestHandleDoctorSurfacesPrincipalsAndScope proves RFC 0107 deliverable 4:
// `daemon doctor` surfaces the configured principals and each one's
// capability/repo scope, and never leaks token material.
func TestHandleDoctorSurfacesPrincipalsAndScope(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.DB(t)
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_a','ident_a','/tmp/repo-a','/tmp/repo-a/.striatum','repo-a',$1,23,'active')`,
		now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	if err := admin.CreatePrincipal(ctx, runner, "prin_alice", "human", "Alice"); err != nil {
		t.Fatalf("create principal: %v", err)
	}
	seedDoctorClient(t, runner, "client_alice", now)
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.client_capabilities (
		  capability_id, client_id, repository_id, capability, granted_at
		) VALUES ('cap_alice','client_alice','repo_a',$1,$2)`,
		string(rpc.CapabilityWrite), now,
	); err != nil {
		t.Fatalf("seed capability: %v", err)
	}
	if err := admin.LinkClientToPrincipal(ctx, runner, "prin_alice", "client_alice"); err != nil {
		t.Fatalf("link client: %v", err)
	}

	result, err := HandleDoctor(ctx, runner, rpc.Envelope{Params: map[string]any{}})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	block, ok := result["principals"].(map[string]any)
	if !ok {
		t.Fatalf("doctor result missing principals block: %#v", result["principals"])
	}
	if block["configured"] != true || block["count"] != 1 {
		t.Fatalf("principals block = %#v, want configured/count 1", block)
	}
	list, ok := block["principals"].([]map[string]any)
	if !ok || len(list) != 1 {
		t.Fatalf("principals list = %#v", block["principals"])
	}
	alice := list[0]
	if alice["principal_id"] != "prin_alice" || alice["kind"] != "human" || alice["display_name"] != "Alice" {
		t.Fatalf("principal entry = %#v", alice)
	}
	caps, ok := alice["capabilities"].([]map[string]any)
	if !ok || len(caps) != 1 || caps[0]["capability"] != "write" || caps[0]["repository_id"] != "repo_a" {
		t.Fatalf("principal capabilities = %#v, want write@repo_a", alice["capabilities"])
	}

	// No token material may appear anywhere in the doctor principals block.
	rendered := strings.ToLower(fmt.Sprintf("%#v", block))
	for _, forbidden := range []string{"token_hash", "token_salt", "secret", "dtok_"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("doctor principals block leaked %q: %s", forbidden, rendered)
		}
	}
}

func seedDoctorClient(t *testing.T, runner db.Runner, clientID string, now time.Time) {
	t.Helper()
	if err := runner.Exec(context.Background(), `
		INSERT INTO striatumd.clients (
		  client_id, client_kind, display_name, token_id, token_hash, token_salt, created_at
		) VALUES ($1,'test',$1,$2,'hash','salt',$3)`,
		clientID, clientID+"_tok", now,
	); err != nil {
		t.Fatalf("seed client %s: %v", clientID, err)
	}
}

