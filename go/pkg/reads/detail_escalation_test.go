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
			"needs principal decision", "open", "2026-05-17T00:00:00Z", nil, validEscalationPayload(),
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
	link, ok := esc["escalation_artifact"].(map[string]any)
	if !ok {
		t.Fatalf("expected escalation artifact summary, got: %#v", esc["escalation_artifact"])
	}
	if link["artifact_id"] != "art_escalation" ||
		link["repo_path"] != "docs/escalations/ESCALATION.md" ||
		link["content_sha256"] != "sha-escalation" ||
		link["linked_at"] != "2026-05-17T00:00:01Z" ||
		link["link_source"] != "artifact.publish" {
		t.Errorf("unexpected escalation artifact link: %#v", link)
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
			"needs principal decision", "open", "2026-05-17T00:00:00Z", nil, validEscalationPayload(),
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

func TestEscalationProjectionSuppressesStaleArtifactPayloadLink(t *testing.T) {
	row := map[string]any{
		"escalation_id":         "blk_1",
		"blocker_id":            "blk_1",
		"run_id":                "run_1",
		"job_id":                "job_1",
		"workflow_job_id":       "build",
		"session_id":            "sess_1",
		"session_role_id":       "implementer",
		"session_lane_id":       "codex",
		"severity":              "blocked",
		"class":                 "missing_authority",
		"description":           "needs principal decision",
		"state":                 "open",
		"created_at":            "2026-05-17T00:00:00Z",
		"resolved_at":           nil,
		"payload_json":          validEscalationPayloadWithHash("sha-stale"),
		"linked_artifact_id":    "art_escalation",
		"linked_repo_path":      "docs/escalations/ESCALATION.md",
		"linked_content_sha256": "sha-escalation",
		"inbox_state":           "pending",
		"viewed_at":             nil,
		"decision_artifact_id":  nil,
		"resolution_note":       nil,
	}

	escalations := shapeEscalations([]map[string]any{row})
	if escalations[0]["escalation_artifact"] != nil {
		t.Fatalf("expected stale escalation artifact link to be suppressed, got %#v", escalations[0]["escalation_artifact"])
	}
	payload := objectOrEmpty(escalations[0]["payload"])
	if payload["escalation_artifact"] == nil {
		t.Fatalf("expected raw payload to remain available, got %#v", payload)
	}
}

func validEscalationPayload() map[string]any {
	return validEscalationPayloadWithHash("sha-escalation")
}

func validEscalationPayloadWithHash(contentSha256 string) map[string]any {
	return map[string]any{
		"seed": "blk_1",
		"escalation_artifact": map[string]any{
			"artifact_id":    "art_escalation",
			"repo_path":      "docs/escalations/ESCALATION.md",
			"content_sha256": contentSha256,
			"linked_at":      "2026-05-17T00:00:01Z",
			"link_source":    "artifact.publish",
		},
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
