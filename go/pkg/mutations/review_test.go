package mutations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestOverrideVerdictRecoversCompletedReviewWithoutPriorVerdict(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "completed"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_recover",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept",
			"rationale":     "published artifact was reviewed manually",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "recovered" {
		t.Fatalf("status = %#v, want recovered", result["status"])
	}
	if result["previous_verdict"] != nil {
		t.Fatalf("previous_verdict = %#v, want nil", result["previous_verdict"])
	}
	if tx.latestVerdict != "accept" {
		t.Fatalf("latest verdict = %q, want accept", tx.latestVerdict)
	}
	if tx.downstreamState != "queued" {
		t.Fatalf("downstream state = %q, want queued", tx.downstreamState)
	}
	if !tx.sawEvent("verdict.overridden") {
		t.Fatalf("verdict.overridden event not appended: %#v", tx.events)
	}
	if !tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("downstream enqueue event not appended: %#v", tx.events)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("transaction commit/rollback = %v/%v", tx.committed, tx.rolledBack)
	}
}

func TestOverrideVerdictCompletesWaitingHumanAndEnqueuesDownstream(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "waiting_human"
	tx.previousVerdict = "needs_revision"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_waiting",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept_with_findings",
			"rationale":     "operator accepted the revision path",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "overridden" {
		t.Fatalf("status = %#v, want overridden", result["status"])
	}
	if result["previous_verdict"] != "needs_revision" {
		t.Fatalf("previous_verdict = %#v", result["previous_verdict"])
	}
	if result["resolved_blockers"] != 1 {
		t.Fatalf("resolved_blockers = %#v, want 1", result["resolved_blockers"])
	}
	if tx.reviewState != "completed" {
		t.Fatalf("review state = %q, want completed", tx.reviewState)
	}
	if tx.downstreamState != "queued" {
		t.Fatalf("downstream state = %q, want queued", tx.downstreamState)
	}
	if !tx.sawEvent("job.completed") {
		t.Fatalf("job.completed event not appended for waiting_human override: %#v", tx.events)
	}
	if !tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("downstream enqueue event not appended: %#v", tx.events)
	}
}

func TestOverrideVerdictAlreadyAcceptingDoesNotMutate(t *testing.T) {
	tx := newReviewOverrideFakeTx()
	tx.reviewState = "completed"
	tx.previousVerdict = "accept"
	runner := &reviewOverrideFakeRunner{tx: tx}

	result, err := HandleOverrideVerdict(context.Background(), runner, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_override_already",
		Method:        "review.override",
		Params: map[string]any{
			"repository_id": "repo_1",
			"session_id":    "sess_override",
			"job_id":        "job_review",
			"verdict":       "accept",
			"rationale":     "no change needed",
		},
	})
	if err != nil {
		t.Fatalf("HandleOverrideVerdict: %v", err)
	}
	if result["status"] != "already_accepting" {
		t.Fatalf("status = %#v, want already_accepting", result["status"])
	}
	if tx.insertedVerdicts != 0 {
		t.Fatalf("inserted verdicts = %d, want 0", tx.insertedVerdicts)
	}
	if tx.downstreamState != "blocked" {
		t.Fatalf("downstream state = %q, want blocked", tx.downstreamState)
	}
	if tx.sawEvent("queue.message_enqueued") {
		t.Fatalf("unexpected downstream enqueue event: %#v", tx.events)
	}
}

type reviewOverrideFakeRunner struct {
	tx *reviewOverrideFakeTx
}

func (r *reviewOverrideFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec outside tx")
}

func (r *reviewOverrideFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return reviewOverrideFakeRow{err: errors.New("unexpected runner query row outside tx")}
}

func (r *reviewOverrideFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected runner query scalar outside tx")
}

func (r *reviewOverrideFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type reviewOverrideFakeTx struct {
	reviewState      string
	downstreamState  string
	previousVerdict  string
	latestVerdict    string
	insertedVerdicts int
	events           []string
	nextEvent        int64
	committed        bool
	rolledBack       bool
}

func newReviewOverrideFakeTx() *reviewOverrideFakeTx {
	return &reviewOverrideFakeTx{
		reviewState:     "completed",
		downstreamState: "blocked",
	}
}

func (tx *reviewOverrideFakeTx) Exec(_ context.Context, sql string, args ...any) error {
	switch {
	case strings.Contains(sql, "INSERT INTO striatumd.verdicts"):
		tx.insertedVerdicts++
		tx.latestVerdict = args[5].(string)
	case strings.Contains(sql, "UPDATE striatumd.jobs") && strings.Contains(sql, "SET state = 'completed'"):
		tx.reviewState = "completed"
	case strings.Contains(sql, "UPDATE striatumd.jobs") && strings.Contains(sql, "SET state = 'queued'"):
		tx.downstreamState = "queued"
	case strings.Contains(sql, "INSERT INTO striatumd.events"):
		tx.events = append(tx.events, args[3].(string))
	}
	return nil
}

func (tx *reviewOverrideFakeTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return reviewOverrideFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		tx.nextEvent++
		return reviewOverrideFakeRow{values: []any{tx.nextEvent}}
	default:
		return reviewOverrideFakeRow{err: errors.New("unexpected query row: " + sql)}
	}
}

func (tx *reviewOverrideFakeTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected query scalar")
}

func (tx *reviewOverrideFakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return tx.queryJobs(sql, args...), nil
	case strings.Contains(sql, "FROM striatumd.sessions"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"session_id": "sess_override",
			"run_id":     "run_1",
			"state":      "active",
			"role_id":    "reviewer",
			"lane_id":    "codex",
		}}), nil
	case strings.Contains(sql, "SELECT 1 FROM striatumd.verdicts"):
		return runPrepareRowsFromMaps(nil), nil
	case strings.Contains(sql, "SELECT * FROM striatumd.verdicts"):
		if tx.previousVerdict == "" {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{
			"verdict_id":           "verdict_previous",
			"verdict":              tx.previousVerdict,
			"findings_artifact_id": nil,
		}}), nil
	case strings.Contains(sql, "SELECT verdict FROM striatumd.verdicts"):
		if tx.latestVerdict == "" {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{"verdict": tx.latestVerdict}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":               "run_1",
			"state":                "running",
			"workflow_snapshot_id": "snap_1",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.workflow_snapshots"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"workflow_snapshot_id": "snap_1",
			"workflow_json": map[string]any{
				"jobs": []any{
					map[string]any{"id": "review", "type": "review"},
				},
			},
		}}), nil
	case strings.Contains(sql, "FROM striatumd.blockers"):
		return runPrepareRowsFromMaps([]map[string]any{{"blocker_id": "blk_1"}}), nil
	case strings.Contains(sql, "FROM striatumd.job_dependencies"):
		return tx.queryDependencies(sql, args...), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *reviewOverrideFakeTx) queryJobs(sql string, args ...any) pgx.Rows {
	if strings.Contains(sql, "state = 'failed'") {
		return runPrepareRowsFromMaps(nil)
	}
	if strings.Contains(sql, "state NOT IN") {
		if tx.downstreamState == "completed" || tx.downstreamState == "skipped" || tx.downstreamState == "canceled" {
			return runPrepareRowsFromMaps(nil)
		}
		return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_apply"}})
	}
	if strings.Contains(sql, "state = 'completed'") {
		return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_review"}})
	}
	jobID := args[1].(string)
	if jobID == "job_review" {
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":             "job_review",
			"run_id":             "run_1",
			"workflow_job_id":    "review",
			"job_type":           "review",
			"state":              tx.reviewState,
			"current_message_id": "msg_review",
		}})
	}
	return runPrepareRowsFromMaps([]map[string]any{{
		"job_id":             "job_apply",
		"run_id":             "run_1",
		"workflow_job_id":    "apply",
		"job_type":           "synthesis",
		"state":              tx.downstreamState,
		"role_id":            "author",
		"lane_selector_json": map[string]any{},
		"max_attempts":       1,
	}})
}

func (tx *reviewOverrideFakeTx) queryDependencies(sql string, args ...any) pgx.Rows {
	if strings.Contains(sql, "depends_on_job_id") {
		if args[1] == "job_review" {
			return runPrepareRowsFromMaps([]map[string]any{{"job_id": "job_apply"}})
		}
		return runPrepareRowsFromMaps(nil)
	}
	if args[1] == "job_apply" {
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":            "job_apply",
			"depends_on_job_id": "job_review",
			"gate_json": map[string]any{
				"requires_verdict": []any{"accept", "accept_with_findings"},
			},
		}})
	}
	return runPrepareRowsFromMaps(nil)
}

func (tx *reviewOverrideFakeTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *reviewOverrideFakeTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *reviewOverrideFakeTx) sawEvent(eventType string) bool {
	for _, event := range tx.events {
		if event == eventType {
			return true
		}
	}
	return false
}

type reviewOverrideFakeRow struct {
	values []any
	err    error
}

func (r reviewOverrideFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *int64:
			*target = value.(int64)
		case **string:
			if value == nil {
				*target = nil
			} else {
				text := value.(string)
				*target = &text
			}
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
