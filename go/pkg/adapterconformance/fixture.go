package adapterconformance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5/pgconn"
)

// seedExec runs a fixture INSERT, retrying a transient Postgres deadlock
// (SQLSTATE 40P01) a few times. RFC 0101 Phase 0a (#137): with the
// InterrogationReady handshake removed, the agent polls work.await_packet
// CONCURRENTLY with the harness seeding the interrogator attestation. The raw
// fixture INSERTs take FK row-share locks on runs/sessions that can momentarily
// race the agent's claim transaction (which takes FOR UPDATE on the same rows);
// that is a pure test-seeding race, NOT the interrogation deadlock under test
// (which is fixed in the daemon handlers and proven by
// TestInterrogationAwaitDeadlock). A bounded retry keeps the fixture robust
// without masking any product behavior.
func seedExec(t *testing.T, ctx context.Context, runner db.Runner, what, sql string, args ...any) {
	t.Helper()
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		err := runner.Exec(ctx, sql, args...)
		if err == nil {
			return
		}
		var pgErr *pgconn.PgError
		if attempt < attempts-1 && errors.As(err, &pgErr) && pgErr.Code == "40P01" {
			time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
			continue
		}
		t.Fatalf("%s: %v", what, err)
	}
}

// Fixture is a validation-passing single-job workflow seeded into a temp target
// repo for the live conformance runner (DESIGN §1.3 fixture.go). It is a
// document_only/draft job — NOT a verdict-capable type — so work.complete
// finalizes it; its write_scope carries allowed_paths (no repo_write mode) so
// artifact.publish path checks pass while write-scope-clean stays a no-op (no
// git needed). The single expected artifact is a front-matter progress_note the
// happy-path agent publishes, exercising the publisher's front-matter
// validation (the real C8 contract).
type Fixture struct {
	RepositoryID string
	RunID        string
	Role         string
	Lane         string
	JobID        string
	RepoRoot     string

	// Artifact is the single artifact the happy path publishes.
	ArtifactKind        string
	ArtifactLogicalName string
	ArtifactRepoPath    string // repo-relative
	ArtifactAbsPath     string // absolute, written to disk

	// Interrogation support (C7): when SeedInterrogator is used, these identify
	// a separate interrogator session that can open an interrogation against the
	// agent's (target) session.
	InterrogatorSessionID string
}

const (
	fixtureRepoIdentity = "conformance-target"
	fixtureWorkflowID   = "conformance-wf"
	fixtureWorkflowJob  = "draft_doc"
)

// SeedFixture creates the temp target repo, seeds the repository + run +
// workflow snapshot (declaring the role+lane the agent registers) + a single
// queued draft job with a pending work message, and writes the expected
// artifact file to disk. The job is interrogable so C7 can run; for the happy
// path the agent answers one interrogation. The repository_id matches the
// harness token scope.
func SeedFixture(t *testing.T, ctx context.Context, runner db.Runner, repositoryID string) *Fixture {
	t.Helper()

	repoRoot := t.TempDir()
	role := "author"
	lane := "claude_code"
	runID := "run_" + repositoryID
	jobID := "job_" + repositoryID

	docDir := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	artifactRepoPath := "docs/conformance-note.md"
	artifactAbs := filepath.Join(repoRoot, artifactRepoPath)
	if err := os.WriteFile(artifactAbs, []byte(fixtureArtifactBody()), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	now := time.Now().UTC()

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'conformance',$5,16,'active')`,
		repositoryID, fixtureRepoIdentity, repoRoot, filepath.Join(repoRoot, ".striatum"), now,
	); err != nil {
		t.Fatalf("seed repository: %v", err)
	}

	writeScope := map[string]any{
		"mode":          "document_only",
		"allowed_paths": []any{"docs"},
	}
	expectedArtifacts := []any{
		map[string]any{
			"logical_name": "conformance_note",
			"kind":         "progress_note",
			"path":         artifactRepoPath,
			"required":     true,
		},
	}
	workflow := map[string]any{
		"workflow_id": fixtureWorkflowID,
		"roles": map[string]any{
			role: map[string]any{"summary": "draft a small document"},
		},
		"lanes": map[string]any{
			lane: map[string]any{
				"display_model": "Claude",
				"capabilities":  []any{"claim", "write", "interrogate"},
			},
		},
		"jobs": []any{
			map[string]any{
				"id":           fixtureWorkflowJob,
				"interrogable": true,
			},
		},
	}

	seedFixtureRun(t, ctx, runner, repositoryID, runID, workflow)
	seedFixtureJob(t, ctx, runner, repositoryID, runID, jobID, role, lane, writeScope, expectedArtifacts, now)

	return &Fixture{
		RepositoryID:        repositoryID,
		RunID:               runID,
		Role:                role,
		Lane:                lane,
		JobID:               jobID,
		RepoRoot:            repoRoot,
		ArtifactKind:        "progress_note",
		ArtifactLogicalName: "conformance_note",
		ArtifactRepoPath:    artifactRepoPath,
		ArtifactAbsPath:     artifactAbs,
	}
}

// SeedInterrogator seeds a separate, interrogate-capable interrogator session
// (active + attested via the current process pid) so it can open+ask an
// interrogation against the agent's target session for C7. It must be called
// after the agent has registered its (target) session so requireLiveTarget's
// liveness checks have a target to find. The target session is attested too.
func (f *Fixture) SeedInterrogator(t *testing.T, ctx context.Context, runner db.Runner, targetSessionID string) {
	t.Helper()
	interrogator := "sess_interrogator_" + f.RepositoryID
	f.InterrogatorSessionID = interrogator
	seedFixtureSession(t, ctx, runner, f.RepositoryID, f.RunID, interrogator, "author", f.Lane,
		[]string{"interrogate"}, 2)
	// Attest both the interrogator and the target so requireLiveTarget passes.
	attestFixtureSession(t, ctx, runner, f.RepositoryID, f.RunID, interrogator, f.Lane)
	attestFixtureSession(t, ctx, runner, f.RepositoryID, f.RunID, targetSessionID, f.Lane)
}

func seedFixtureRun(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, runID string, workflow map[string]any) {
	t.Helper()
	now := time.Now().UTC()
	workflowArg, err := db.JSONBArg(runner, workflow)
	if err != nil {
		t.Fatalf("encode workflow: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,$3,'sha',$4::jsonb,$5)`,
		repositoryID, "snap_"+runID, fixtureWorkflowID, workflowArg, now); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
		) VALUES ($1,$2,$3,(SELECT repo_root FROM striatumd.repositories WHERE repository_id=$1),'running',$4)`,
		repositoryID, runID, "snap_"+runID, now); err != nil {
		t.Fatalf("seed run: %v", err)
	}
}

func seedFixtureJob(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, runID, jobID, role, lane string, writeScope map[string]any, expectedArtifacts []any, now time.Time) {
	t.Helper()
	writeScopeArg, err := db.JSONBArg(runner, writeScope)
	if err != nil {
		t.Fatalf("encode write_scope: %v", err)
	}
	expectedArg, err := db.JSONBArg(runner, expectedArtifacts)
	if err != nil {
		t.Fatalf("encode expected_artifacts: %v", err)
	}
	laneSelectorArg, err := db.JSONBArg(runner, map[string]any{"lane_id": lane})
	if err != nil {
		t.Fatalf("encode lane_selector: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, write_scope_json, expected_artifacts_json,
		  lane_selector_json, created_at
		) VALUES ($1,$2,$3,$4,1,'queued',$5,'Conformance draft','draft',$6,$7::jsonb,$8::jsonb,$9::jsonb,$10)`,
		repositoryID, jobID, runID, fixtureWorkflowJob, role, "idem_"+jobID,
		writeScopeArg, expectedArg, laneSelectorArg, now); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','pending',0,$5,$6,$7,$7)`,
		repositoryID, "msg_"+jobID, runID, jobID, role, lane, now); err != nil {
		t.Fatalf("seed queue message: %v", err)
	}
}

func seedFixtureSession(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, runID, sessionID, role, lane string, caps []string, ordinal int) {
	t.Helper()
	now := time.Now().UTC()
	capsArg, err := db.JSONBArg(runner, caps)
	if err != nil {
		t.Fatalf("encode caps: %v", err)
	}
	seedExec(t, ctx, runner, "seed session "+sessionID, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
		  capabilities_json, state, registered_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,'active',$9)`,
		repositoryID, sessionID, runID, role, lane, sessionID+"-slug", ordinal, capsArg, now)
}

// attestFixtureSession seeds an attached supervisor + pointer + daemon
// supervisor for the session, using the current process pid so the live-target
// health check (lanehealth) treats it as a live, attested target — mirroring
// the in-tree intgAttest helper used by the mutations integration tests.
func attestFixtureSession(t *testing.T, ctx context.Context, runner db.Runner, repositoryID, runID, sessionID, lane string) {
	t.Helper()
	now := time.Now().UTC()
	supID := "sup_" + sessionID
	pid := os.Getpid()
	seedExec(t, ctx, runner, "attest supervisor "+sessionID, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ($1,$2,$3,$4,$5,'[]'::jsonb,'/tmp','/tmp/scratch',$6,'attached',$7)`,
		repositoryID, supID, runID, sessionID, lane, pid, now)
	dsupID := "dsup_" + sessionID
	seedExec(t, ctx, runner, "attest pointer "+sessionID, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'','attached',$7,'{}'::jsonb)`,
		repositoryID, supID, dsupID, runID, sessionID, pid, now)
	seedExec(t, ctx, runner, "attest daemon supervisor "+sessionID, `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
		  daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
		  pid_start_time, state, started_at, heartbeat_at
		) VALUES ($1,$2,$3,$4,$5,'inst',$6,'[]'::jsonb,'sha','/tmp',$7,'','attached',$8,$8)`,
		dsupID, repositoryID, runID, sessionID, supID, lane, pid, now)
}

// fixtureArtifactBody is the progress_note the happy path publishes: a valid
// striatum.progress_note.v1 front-matter block with no `author:` line (the
// byline check only fires on a present title-block author line), so the
// publisher validates front matter without an attestation-derived byline.
func fixtureArtifactBody() string {
	return fmt.Sprintf(`---
schema_version: "striatum.progress_note.v1"
artifact_kind: "progress_note"
note_date: "%s"
session_slug: "conformance"
related_plan: null
related_brief: null
retrieval_priority: "low"
---

# Conformance progress note

Seeded by the RFC 0101 Layer 2 fake-agent conformance runner. This document
exists only to exercise the artifact.publish front-matter contract (C8).
`, time.Now().UTC().Format("2006-01-02"))
}
