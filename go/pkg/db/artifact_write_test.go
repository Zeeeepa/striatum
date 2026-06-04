package db

import (
	"context"
	"strings"
	"testing"
)

// recordingArtifactTx captures the SQL of the single write AppendArtifactInTx
// issues, and which method carried it, so the routing decision can be asserted
// without a database.
type recordingArtifactTx struct {
	execSQL   string
	scalarSQL string
}

func (t *recordingArtifactTx) Exec(_ context.Context, sql string, _ ...any) error {
	t.execSQL = sql
	return nil
}
func (t *recordingArtifactTx) QueryRow(context.Context, string, ...any) Row { return nil }
func (t *recordingArtifactTx) QueryScalar(_ context.Context, sql string, _ ...any) (string, error) {
	t.scalarSQL = sql
	return "", nil
}
func (t *recordingArtifactTx) Commit(context.Context) error   { return nil }
func (t *recordingArtifactTx) Rollback(context.Context) error { return nil }

// TestAppendArtifactRoutesByPhase is the RFC 0110 §7 routing gate: below
// audit_artifacts the artifact write is a direct INSERT; at or above it routes
// through the owner-owned SECURITY DEFINER append_artifact_row.
func TestAppendArtifactRoutesByPhase(t *testing.T) {
	t.Cleanup(func() { SetActiveWriteBoundary(PhaseNone) })
	row := ArtifactRow{RepositoryID: "repo", ArtifactID: "art", RunID: "run", LogicalName: "log"}

	for _, tc := range []struct {
		phase  WriteBoundaryPhase
		wantSD bool
	}{
		{PhaseNone, false},
		{PhaseAuditOnly, false},
		{PhaseAuditArtifacts, true},
		{PhaseFull, true},
	} {
		SetActiveWriteBoundary(tc.phase)
		tx := &recordingArtifactTx{}
		if err := AppendArtifactInTx(context.Background(), tx, row); err != nil {
			t.Fatalf("phase %s: %v", tc.phase, err)
		}
		if tc.wantSD {
			if !strings.Contains(tx.scalarSQL, "striatumd.append_artifact_row") {
				t.Fatalf("phase %s: expected SD routing, got exec=%q scalar=%q", tc.phase, tx.execSQL, tx.scalarSQL)
			}
			if tx.execSQL != "" {
				t.Fatalf("phase %s: SD routing must not also direct-INSERT (exec=%q)", tc.phase, tx.execSQL)
			}
		} else {
			if !strings.Contains(tx.execSQL, "INSERT INTO striatumd.artifacts") {
				t.Fatalf("phase %s: expected direct INSERT, got exec=%q scalar=%q", tc.phase, tx.execSQL, tx.scalarSQL)
			}
			if tx.scalarSQL != "" {
				t.Fatalf("phase %s: pre-P1 must not call the SD function (scalar=%q)", tc.phase, tx.scalarSQL)
			}
		}
	}
}
