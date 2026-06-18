package mutations

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// openHookConversation seeds a repo+run with three active participant sessions
// and opens a conversation declaring a post_dialog_hook that delivers a packet
// to the coordinator participant on close. Returns conversation id + the seeded
// session ids (a is the coordinator/deliver_to).
func openHookConversation(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string, maxRounds int) (cid, a, b, c string) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"jobs": []any{}})
	a, b, c = "pdh_a", "pdh_b", "pdh_c"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, a, "coordinator", "claude", nil, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, b, "participant", "codex", nil, "active", 2)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, c, "participant", "gemini", nil, "active", 3)
	open, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"participant_session_ids": []string{a, b, c},
		"topic":                   "prune",
		"max_rounds":              maxRounds,
		"post_dialog_hook": map[string]any{
			"deliver_to":  a,
			"packet_type": "prune",
		},
	}))
	if err != nil {
		t.Fatalf("open with hook: %v", err)
	}
	return fmt.Sprint(open["conversation_id"]), a, b, c
}

// TestPostDialogHookEmitsOnExplicitClose proves conversation.close with the hook
// declared emits exactly one packet to the coordinator session, carrying the
// participant session ids + a transcript ref, and that the coordinator's await
// loop delivers it (emit-before-teardown — the participants remain active).
func TestPostDialogHookEmitsOnExplicitClose(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	cid, a, b, c := openHookConversation(t, ctx, runner, "repo_pdh_close", "run_pdh_close", 3)

	// No hook packet pending before close.
	if pkt, err := deliverPendingPostDialogHook(ctx, runner, "repo_pdh_close", a); err != nil || pkt != nil {
		t.Fatalf("pre-close hook delivery = %v, %v; want nil, nil", pkt, err)
	}

	res, err := HandleConversationClose(ctx, runner, intgEnv("repo_pdh_close", map[string]any{
		"session_id": b, "conversation_id": cid,
	}))
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if res["post_dialog_hook_emitted"] != true {
		t.Fatalf("close result did not report post_dialog_hook_emitted: %v", res)
	}

	pkt, err := deliverPendingPostDialogHook(ctx, runner, "repo_pdh_close", a)
	if err != nil {
		t.Fatalf("deliver hook to coordinator: %v", err)
	}
	if pkt == nil {
		t.Fatalf("coordinator received no post_dialog_hook packet")
	}
	if pkt["status"] != "post_dialog_hook" || pkt["packet_type"] != "prune" {
		t.Fatalf("hook packet shape = %v", pkt)
	}
	if fmt.Sprint(pkt["transcript_ref"]) != cid || fmt.Sprint(pkt["conversation_id"]) != cid {
		t.Fatalf("hook transcript_ref/conversation_id mismatch: %v (want %s)", pkt, cid)
	}
	ids := asStringSlice(pkt["participant_session_ids"])
	if len(ids) != 3 || !containsString(ids, a) || !containsString(ids, b) || !containsString(ids, c) {
		t.Fatalf("hook participant_session_ids = %v, want [%s %s %s]", ids, a, b, c)
	}

	// Exactly one emit: a second delivery finds nothing pending.
	if again, err := deliverPendingPostDialogHook(ctx, runner, "repo_pdh_close", a); err != nil || again != nil {
		t.Fatalf("second hook delivery = %v, %v; want nil, nil (exactly-one emit)", again, err)
	}

	// Emit-before-teardown: the participants are still active so a coordinator
	// could fan out interrogation.open against them. conversation.close does not
	// close participant sessions in this slice.
	for _, sid := range []string{a, b, c} {
		state, err := sessionState(ctx, runner, "repo_pdh_close", sid)
		if err != nil {
			t.Fatalf("sessionState(%s): %v", sid, err)
		}
		if state != "active" {
			t.Fatalf("participant %s state = %q after close, want active (preserved-context window)", sid, state)
		}
	}
}

// TestPostDialogHookEmitsOnAutoClose proves the hook also fires on the auto-close
// path (max_rounds reached in conversation.say), not just explicit close.
func TestPostDialogHookEmitsOnAutoClose(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	cid, a, b, c := openHookConversation(t, ctx, runner, "repo_pdh_auto", "run_pdh_auto", 1)

	// One full round (A,B,C) reaches max_rounds=1 and auto-closes on C's say.
	order := []string{a, b, c}
	for i, who := range order {
		res, err := HandleConversationSay(ctx, runner, intgEnv("repo_pdh_auto", map[string]any{
			"session_id": who, "conversation_id": cid, "body": fmt.Sprintf("turn %d", i),
		}))
		if err != nil {
			t.Fatalf("say %d (%s): %v", i, who, err)
		}
		if i == len(order)-1 && res["state"] != "closed" {
			t.Fatalf("final say state = %v, want closed", res["state"])
		}
	}

	pkt, err := deliverPendingPostDialogHook(ctx, runner, "repo_pdh_auto", a)
	if err != nil {
		t.Fatalf("deliver hook after auto-close: %v", err)
	}
	if pkt == nil || pkt["packet_type"] != "prune" {
		t.Fatalf("auto-close did not emit the hook packet: %v", pkt)
	}
}

// TestPostDialogHookSilentWhenAbsent proves a conversation without a declared
// hook emits nothing on close.
func TestPostDialogHookSilentWhenAbsent(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_pdh_none", "run_pdh_none"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"jobs": []any{}})
	a, b := "none_a", "none_b"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, a, "participant", "claude", nil, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, b, "participant", "codex", nil, "active", 2)
	open, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
		"participant_session_ids": []string{a, b}, "topic": "t", "max_rounds": 2,
	}))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cid := fmt.Sprint(open["conversation_id"])
	res, err := HandleConversationClose(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": a, "conversation_id": cid,
	}))
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, reported := res["post_dialog_hook_emitted"]; reported {
		t.Fatalf("close reported a hook emit on a hookless conversation: %v", res)
	}
	if pkt, err := deliverPendingPostDialogHook(ctx, runner, repoID, a); err != nil || pkt != nil {
		t.Fatalf("hookless conversation delivered a packet: %v, %v", pkt, err)
	}
}

// TestPostDialogHookValidation covers the conversation.open validation of the
// post_dialog_hook declaration.
func TestPostDialogHookValidation(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID, runID := "repo_pdh_val", "run_pdh_val"
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"jobs": []any{}})
	a, b := "val_a", "val_b"
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, a, "participant", "claude", nil, "active", 1)
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, b, "participant", "codex", nil, "active", 2)

	for _, tc := range []struct {
		name string
		hook any
	}{
		{"not an object", "nope"},
		{"missing deliver_to", map[string]any{"packet_type": "prune"}},
		{"missing packet_type", map[string]any{"deliver_to": a}},
		{"deliver_to not a participant", map[string]any{"deliver_to": "stranger", "packet_type": "prune"}},
		{"empty deliver_to", map[string]any{"deliver_to": "", "packet_type": "prune"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := HandleConversationOpen(ctx, runner, intgEnv(repoID, map[string]any{
				"participant_session_ids": []string{a, b},
				"max_rounds":              2,
				"post_dialog_hook":        tc.hook,
			}))
			if !isRPCCode(err, "schema_invalid") {
				t.Fatalf("open with %s: want schema_invalid, got %v", tc.name, err)
			}
		})
	}
}
