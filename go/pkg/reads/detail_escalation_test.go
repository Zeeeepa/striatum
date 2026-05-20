package reads

import (
	"context"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

func TestHandleEscalationList(t *testing.T) {
	fake := &detailEscalationFake{
		fields: []string{
			"escalation_id", "blocker_id", "run_id", "job_id", "workflow_job_id",
			"session_id", "session_role_id", "session_lane_id", "severity", "class",
			"description", "state", "created_at", "resolved_at", "payload_json",
			"linked_artifact_id", "linked_repo_path", "linked_content_sha256",
			"inbox_state", "viewed_at", "decision_artifact_id", "resolution_note",
		},
		rows: [][]any{{
			"blk_1", "blk_1", "run_1", "job_1", "build",
			"sess_1", "implementer", "codex", "error", "missing_authority",
			"needs principal decision", "open", "2026-05-17T00:00:00Z", nil, map[string]any{"seed": "blk_1"},
			"art_escalation", "docs/escalations/ESCALATION.md", "sha-escalation",
			"pending", nil, nil, nil,
		}},
	}

	response, err := HandleEscalationList(context.Background(), fake, rpc.Envelope{Params: map[string]any{
		"repository_id": "repo_1",
	}})
	if err != nil {
		t.Fatalf("HandleEscalationList failed: %v", err)
	}

	escalations, ok := response["escalations"].([]map[string]any)
	if !ok || len(escalations) != 1 {
		t.Fatalf("expected 1 escalation, got: %#v", response)
	}

	esc := escalations[0]
	if esc["escalation_id"] != "blk_1" || esc["inbox_state"] != "pending" {
		t.Errorf("unexpected escalation state: %#v", esc)
	}
}

func TestHandleEscalationShowMarksViewed(t *testing.T) {
	fake := &detailEscalationFake{
		fields: []string{
			"escalation_id", "blocker_id", "run_id", "job_id", "workflow_job_id",
			"session_id", "session_role_id", "session_lane_id", "severity", "class",
			"description", "state", "created_at", "resolved_at", "payload_json",
			"linked_artifact_id", "linked_repo_path", "linked_content_sha256",
			"inbox_state", "viewed_at", "decision_artifact_id", "resolution_note",
		},
		rows: [][]any{{
			"blk_1", "blk_1", "run_1", "job_1", "build",
			"sess_1", "implementer", "codex", "error", "missing_authority",
			"needs principal decision", "open", "2026-05-17T00:00:00Z", nil, map[string]any{"seed": "blk_1"},
			"art_escalation", "docs/escalations/ESCALATION.md", "sha-escalation",
			"viewed", "2026-05-20T04:26:00Z", nil, nil,
		}},
	}

	response, err := HandleEscalationShow(context.Background(), fake, rpc.Envelope{Params: map[string]any{
		"repository_id": "repo_1",
		"escalation_id": "blk_1",
	}})
	if err != nil {
		t.Fatalf("HandleEscalationShow failed: %v", err)
	}

	esc, ok := response["escalation"].(map[string]any)
	if !ok {
		t.Fatalf("expected escalation map, got: %#v", response)
	}

	if esc["escalation_id"] != "blk_1" || esc["inbox_state"] != "viewed" {
		t.Errorf("unexpected escalation: %#v", esc)
	}

	if len(fake.execSQL) != 1 || !strings.Contains(fake.execSQL[0], "UPDATE striatumd.escalation_inbox") {
		t.Errorf("expected escalation_inbox update to be executed, execs: %v", fake.execSQL)
	}
}

type detailEscalationFake struct {
	fields  []string
	rows    [][]any
	execSQL []string
}

func (f *detailEscalationFake) Exec(_ context.Context, sql string, _ ...any) error {
	f.execSQL = append(f.execSQL, sql)
	return nil
}

func (f *detailEscalationFake) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (f *detailEscalationFake) QueryScalar(_ context.Context, _ string, _ ...any) (string, error) {
	return "", nil
}

func (f *detailEscalationFake) BeginTx(_ context.Context) (db.TxRunner, error) {
	return nil, nil
}

func (f *detailEscalationFake) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakeRows{
		fields: f.fields,
		rows:   f.rows,
	}, nil
}
