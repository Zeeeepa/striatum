package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleRunSummary mirrors reads/run_summary.py — a redaction-safe
// snapshot of the run's jobs, artifacts, verdicts, and a doctor block.
// In the Go port the doctor block calls HandleDoctor for parity with
// the Python implementation post-v1.52.0.
func HandleRunSummary(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.summary requires run_id", nil)
	}

	runs, err := collectRows(ctx, runner,
		`SELECT run_id, workflow_snapshot_id, repo_root, state, branch_name,
		        created_at, started_at, completed_at, stop_reason
		   FROM striatumd.runs WHERE repository_id = $1 AND run_id = $2`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, rpc.NewError("not_found", "run not found", nil)
	}

	jobs, err := collectRows(ctx, runner,
		`SELECT workflow_job_id, job_id, role_id, state, attempt
		   FROM striatumd.jobs
		  WHERE repository_id = $1 AND run_id = $2
		  ORDER BY created_at`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}

	artifacts, err := collectRows(ctx, runner,
		`SELECT artifact_id, job_id, kind, logical_name, path,
		        content_sha256, byline, published_at
		   FROM striatumd.artifacts
		  WHERE repository_id = $1 AND run_id = $2
		  ORDER BY published_at`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}

	verdicts, err := collectRows(ctx, runner,
		`SELECT verdict_id, job_id, session_id, verdict, review_posture,
		        recorded_at
		   FROM striatumd.verdicts
		  WHERE repository_id = $1 AND run_id = $2
		  ORDER BY recorded_at`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}

	doctor, err := HandleDoctor(ctx, runner, envelope)
	if err != nil {
		doctor = map[string]any{"ok": false, "error": err.Error()}
	}

	return map[string]any{
		"run":       runs[0],
		"jobs":      jobs,
		"artifacts": artifacts,
		"verdicts":  verdicts,
		"doctor":    doctor,
	}, nil
}

// HandleEvidenceExport mirrors reads/evidence_export.py — a redacted
// markdown export structure. The Go port returns the rows as data;
// rendering to Markdown stays the CLI's responsibility (matches the
// Python handler which also returns a structured payload).
func HandleEvidenceExport(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	args := []any{repositoryID}
	runWhere := ""
	if runID != "" {
		args = append(args, runID)
		runWhere = " AND run_id = $2"
	}
	runs, err := collectRows(ctx, runner,
		`SELECT run_id, state, branch_name, completed_at
		   FROM striatumd.runs WHERE repository_id = $1`+runWhere+
			` ORDER BY created_at DESC LIMIT 50`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	artifacts, err := collectRows(ctx, runner,
		`SELECT artifact_id, run_id, job_id, kind, logical_name, path,
		        content_sha256, byline, published_at
		   FROM striatumd.artifacts WHERE repository_id = $1`+runWhere+
			` ORDER BY published_at DESC LIMIT 500`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	verdicts, err := collectRows(ctx, runner,
		`SELECT verdict_id, run_id, job_id, verdict, review_posture, recorded_at
		   FROM striatumd.verdicts WHERE repository_id = $1`+runWhere+
			` ORDER BY recorded_at DESC LIMIT 500`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	doctor, _ := HandleDoctor(ctx, runner, envelope)
	return map[string]any{
		"runs":      runs,
		"artifacts": artifacts,
		"verdicts":  verdicts,
		"doctor":    doctor,
	}, nil
}

// HandleCorpusExport mirrors reads/corpus_export.py — exposes the
// redaction-safe corpus rows the augmentation contract consumes.
// V1.7: returns the bundle's manifest + a paged row list. Heavy lifting
// (redaction, redaction-tier compliance) stays in the Python handler
// until the Go port grows the same surface; this Go handler returns the
// raw row payload + structure so the consumer can detect substrate.
func HandleCorpusExport(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	limit, count := limitClause(envelope, 1000)
	rows, err := collectRows(ctx, runner,
		`SELECT artifact_id, run_id, kind, logical_name, path,
		        content_sha256, byline, published_at
		   FROM striatumd.artifacts
		  WHERE repository_id = $1
		  ORDER BY published_at DESC`+limit,
		repositoryID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"corpus_contract_version": 1,
		"repository_id":           repositoryID,
		"row_count":               len(rows),
		"limit":                   count,
		"rows":                    rows,
	}, nil
}
