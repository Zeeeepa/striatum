package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// RFC 0167 P0 / owner bundle 0022 regression guard (the daemon-wide outage of
// 2026-06-25): the bundle REVOKEs table-level SELECT on striatumd.runs from the
// runtime role and re-GRANTs every column EXCEPT created_by_principal_id, so a
// `SELECT *` on runs 42501s for striatumd_rw and wedges every run-scoped
// mutation (claim, run.start, work.complete, run.cancel, worktree anchor, ...).
// rowByID must therefore project the granted columns explicitly for the runs
// table. This guard is hermetic (no PG): it captures the SQL rowByID generates
// and asserts the runs read never selects `*` nor the revoked column, while a
// non-runs table still uses the generic `SELECT *`.

// sqlCaptureQueryer is a minimal queryer that records the SQL rowByID issues and
// returns an empty result set (rowByID's caller turns that into ErrNoRows, which
// is irrelevant — we only assert on the captured SQL).
type sqlCaptureQueryer struct{ sql []string }

func (q *sqlCaptureQueryer) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.sql = append(q.sql, sql)
	return runPrepareRowsFromMaps(nil), nil
}

func TestRowByIDRunsProjectionAvoidsRevokedColumn(t *testing.T) {
	ctx := context.Background()

	q := &sqlCaptureQueryer{}
	_, _ = rowByID(ctx, q, "repo_x", "runs", "run_id", "run_x", true)
	if len(q.sql) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(q.sql))
	}
	got := q.sql[0]

	if strings.Contains(got, "SELECT *") {
		t.Fatalf("runs read must not SELECT * (42501 under the bundle 0022 column grant): %q", got)
	}
	if strings.Contains(got, "created_by_principal_id") {
		t.Fatalf("runs read must not reference the SELECT-revoked created_by_principal_id: %q", got)
	}
	if !strings.Contains(got, "FROM striatumd.runs") {
		t.Fatalf("runs read lost its table target: %q", got)
	}
	if !strings.Contains(got, "FOR UPDATE") {
		t.Fatalf("forUpdate=true must still emit FOR UPDATE: %q", got)
	}
	// Every field the mutation surface reads (state, branch, completion, ...)
	// must be present so existing callers' map reads keep resolving. These are
	// the base/runtime-migration columns (through 0026), which exist at every
	// daemon-supported schema version.
	for _, col := range []string{
		"repository_id", "run_id", "workflow_snapshot_id", "repo_root", "state",
		"branch_name", "branch_base", "branch_confirmed_at", "branch_confirmed_by", "created_at",
		"started_at", "completed_at", "stop_reason", "paused_at", "paused_reason", "cross_repo_run_id",
		"completion_mode", "completion_record_json",
	} {
		if !strings.Contains(got, col) {
			t.Fatalf("runs projection is missing column %q: %q", col, got)
		}
	}
	// created_by_handle_id is a bundle-0022 column no mutation reads from this
	// row; naming it would 42703 on a pre-bundle schema.
	if strings.Contains(got, "created_by_handle_id") {
		t.Fatalf("runs projection must not name the bundle-0022-only created_by_handle_id: %q", got)
	}

	// Other run-scoped tables keep the generic SELECT * (they retain table-level
	// SELECT and have no column-revoke), so the special-case must be runs-only.
	q2 := &sqlCaptureQueryer{}
	_, _ = rowByID(ctx, q2, "repo_x", "jobs", "job_id", "job_x", false)
	if len(q2.sql) != 1 || !strings.Contains(q2.sql[0], "SELECT * FROM striatumd.jobs") {
		t.Fatalf("non-runs table should keep SELECT *, got %v", q2.sql)
	}
}

func TestRunResumeReadsRunsUnderTwoRoleColumnGrant(t *testing.T) {
	ctx := context.Background()
	fx := pgtest.TwoRole(t)
	repoID := "repo_two_role_runs_projection"
	runID := "run_two_role_runs_projection"

	intgSeedRepo(t, ctx, fx.OwnerPool.Runner, repoID)
	intgSeedRun(t, ctx, fx.OwnerPool.Runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"author": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{"capabilities": []any{"write"}}},
	})

	if err := fx.SUTPool.Runner.Exec(ctx, `
		SELECT count(*)
		  FROM (SELECT * FROM striatumd.runs
		         WHERE repository_id = $1 AND run_id = $2) q`,
		repoID, runID); !pgCode(err, "42501") {
		t.Fatalf("precondition: SELECT * over runs must be 42501 under the two-role column grant, got %v", err)
	}

	result, err := HandleRunResume(ctx, fx.SUTPool.Runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("run.resume under two-role runtime grant: %v", err)
	}
	if result["state"] != "running" || result["status"] != "not_paused" {
		t.Fatalf("run.resume result = %#v, want running/not_paused", result)
	}

	run, err := oneRow(ctx, fx.OwnerPool.Runner, `
		SELECT state, paused_reason
		  FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run: %v", err)
	}
	if run["state"] != "running" || run["paused_reason"] != nil {
		t.Fatalf("run row = %#v, want running and not paused", run)
	}
}

func pgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
