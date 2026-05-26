package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestConversationRoundRobin exercises the RFC 0086 primitive end to end:
// floor advances round-robin, only the floor-holder may say, delivery is
// floor-derived + idempotent (crash-safe), and the conversation auto-closes at
// max_rounds.
func TestConversationRoundRobin(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_conv"
	runID := "run_conv"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"jobs": []any{}})
	a, b, c := "sess_a", "sess_b", "sess_c"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, a, "participant", "claude", nil, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, b, "participant", "codex", nil, "active", 2)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, c, "participant", "gemini", nil, "active", 3)

	open, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"participant_session_ids": []string{a, b, c}, "topic": "t", "max_rounds": 2,
	}))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cid := fmt.Sprint(open["conversation_id"])

	// Floor starts at A: A is delivered its turn; B is not.
	mustDeliver(t, ctx, runner, repoID, a, cid, true)
	mustDeliver(t, ctx, runner, repoID, b, cid, false)

	// Crash-safety: delivery is idempotent — A sees its turn repeatedly until it
	// says (no consumable message that could be lost on agent crash).
	mustDeliver(t, ctx, runner, repoID, a, cid, true)

	// Only the floor-holder may say.
	if _, err := HandleConversationSay(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": b, "conversation_id": cid, "body": "out of turn",
	})); !isRPCCode(err, "capability_denied") {
		t.Fatalf("non-floor say: want capability_denied, got %v", err)
	}

	// Round-robin: A,B,C,A,B,C -> 2 full rounds -> auto-close on the 6th say.
	order := []string{a, b, c, a, b, c}
	for i, who := range order {
		res, err := HandleConversationSay(ctx, runner, intgEnv(repoID, map[string]any{
			"session_id": who, "conversation_id": cid, "body": fmt.Sprintf("turn %d", i),
		}))
		if err != nil {
			t.Fatalf("say %d (%s): %v", i, who, err)
		}
		wantClosed := i == len(order)-1
		if got := res["state"] == "closed"; got != wantClosed {
			t.Fatalf("say %d state=%v want closed=%v", i, res["state"], wantClosed)
		}
	}

	// After close: no one holds a deliverable floor, and the transcript has 6 turns.
	mustDeliver(t, ctx, runner, repoID, a, cid, false)
	show, err := HandleConversationShow(ctx, runner, intgEnv(repoID, map[string]any{"conversation_id": cid}))
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if show["turn_count"].(int) != 6 {
		t.Fatalf("turn_count = %v, want 6", show["turn_count"])
	}
	if conv, _ := show["conversation"].(map[string]any); fmt.Sprint(conv["state"]) != "closed" {
		t.Fatalf("conversation state = %v, want closed", conv["state"])
	}
}

func mustDeliver(t *testing.T, ctx context.Context, runner db.Runner, repoID, sessionID, cid string, want bool) {
	t.Helper()
	res, err := deliverPendingConversationTurn(ctx, runner, repoID, sessionID)
	if err != nil {
		t.Fatalf("deliver(%s): %v", sessionID, err)
	}
	got := res != nil
	if got != want {
		t.Fatalf("deliver(%s): got=%v want=%v (res=%v)", sessionID, got, want, res)
	}
	if got && fmt.Sprint(res["conversation_id"]) != cid {
		t.Fatalf("deliver(%s): conversation_id=%v want %v", sessionID, res["conversation_id"], cid)
	}
}
