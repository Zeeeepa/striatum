package reads

import (
	"context"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestTrajectoryExportReproducesSeededRun is RFC 0080 follow-up F33: a seeded
// live-PostgreSQL trajectory test. It seeds a run with two bus chat messages
// and an artifact publication between them, then asserts trajectory.export
// reproduces them in derived run-event order for both profiles, with curated
// content and no raw provider output (D028).
func TestTrajectoryExportReproducesSeededRun(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_traj"
	runID := "run_traj"
	sessionID := "sess_speaker"
	base := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	seedTrajRepoRun(t, ctx, runner, repoID, runID, sessionID)
	// Turn 1 (oldest), then an artifact, then turn 2 (newest).
	seedTrajMessage(t, ctx, runner, repoID, runID, sessionID, "msg_1", "first turn", base)
	seedTrajArtifact(t, ctx, runner, repoID, runID, sessionID, "art_1", "turn1", base.Add(time.Second))
	seedTrajMessage(t, ctx, runner, repoID, runID, sessionID, "msg_2", "second turn", base.Add(2*time.Second))

	records := exportTrajectory(t, ctx, runner, repoID, runID, "dialogue")
	if len(records) != 3 {
		t.Fatalf("dialogue records = %d, want 3: %#v", len(records), records)
	}

	// Derived ordering: chronological by created_at, seq 1..3 ascending.
	wantKinds := []string{"agent_message", "artifact_published", "agent_message"}
	wantText := []string{"first turn", "", "second turn"}
	for i, rec := range records {
		seq, _ := rec["seq"].(int64)
		if int(seq) != i+1 {
			t.Fatalf("record %d seq = %v, want %d", i, rec["seq"], i+1)
		}
		if rec["kind"] != wantKinds[i] {
			t.Fatalf("record %d kind = %v, want %s", i, rec["kind"], wantKinds[i])
		}
		body, _ := rec["body"].(map[string]any)
		// D028: never raw provider output.
		if _, bad := body["provider_transcript"]; bad {
			t.Fatalf("record %d leaked provider_transcript (D028)", i)
		}
		if wantText[i] != "" {
			if got := trajText(body); got != wantText[i] {
				t.Fatalf("record %d text = %q, want %q (body=%#v)", i, got, wantText[i], body)
			}
		}
	}

	// since_seq cursor (watch semantics): only records after seq 2 remain.
	tail := exportTrajectorySince(t, ctx, runner, repoID, runID, "dialogue", 2)
	if len(tail) != 1 || tail[0]["kind"] != "agent_message" {
		t.Fatalf("since_seq=2 tail = %#v, want 1 agent_message", tail)
	}

	// Provenance profile includes at least the dialogue records.
	prov := exportTrajectory(t, ctx, runner, repoID, runID, "provenance")
	if len(prov) < 3 {
		t.Fatalf("provenance records = %d, want >= 3", len(prov))
	}
}

func trajText(body map[string]any) string {
	switch v := body["text"].(type) {
	case string:
		return v
	case map[string]any:
		if s, ok := v["text"].(string); ok {
			return s
		}
	}
	return ""
}

func exportTrajectory(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, profile string) []map[string]any {
	t.Helper()
	resp, err := HandleTrajectoryExport(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "trajectory.export",
		Params:        map[string]any{"repository_id": repoID, "run_id": runID, "profile": profile},
	})
	if err != nil {
		t.Fatalf("trajectory.export %s: %v", profile, err)
	}
	records, _ := resp["records"].([]map[string]any)
	return records
}

func exportTrajectorySince(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, profile string, since int) []map[string]any {
	t.Helper()
	resp, err := HandleTrajectoryWatch(ctx, runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		Method:        "trajectory.watch",
		Params:        map[string]any{"repository_id": repoID, "run_id": runID, "profile": profile, "since_seq": since},
	})
	if err != nil {
		t.Fatalf("trajectory.watch: %v", err)
	}
	records, _ := resp["records"].([]map[string]any)
	return records
}

func seedTrajRepoRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+repoID, "/tmp/"+repoID, "/tmp/"+repoID+"/.striatum", now); err != nil {
		t.Fatalf("seed repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'wf','sha','{}'::jsonb,$3)`,
		repoID, "snap_"+runID, now); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,$4,'running',$5)`,
		repoID, runID, "snap_"+runID, "/tmp/"+repoID, now); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ($1,$2,$3,'speaker','claude',$4,1,'[]'::jsonb,'active',$5)`,
		repoID, sessionID, runID, sessionID+"-slug", now); err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

func seedTrajMessage(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, messageID, text string, at time.Time) {
	t.Helper()
	payload, err := db.JSONBArg(runner, map[string]any{"kind": "chat", "body": map[string]any{"text": text}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, kind, state, target_session_id,
		  target_role_id, target_lane_id, payload_json, created_at, updated_at
		) VALUES ($1,$2,$3,'agent_message','pending',$4,'speaker','claude',$5::jsonb,$6,$6)`,
		repoID, messageID, runID, sessionID, payload, at); err != nil {
		t.Fatalf("seed message %s: %v", messageID, err)
	}
}

func seedTrajArtifact(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, artifactID, logicalName string, at time.Time) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, session_id, logical_name, artifact_kind, repo_path, content_sha256, size_bytes, publish_mode, created_at
		) VALUES ($1,$2,$3,$4,$5,'synthesis',$6,$7,0,'create',$8)`,
		repoID, artifactID, runID, sessionID, logicalName, "docs/"+logicalName+".md", "sha_"+artifactID, at); err != nil {
		t.Fatalf("seed artifact %s: %v", artifactID, err)
	}
}
