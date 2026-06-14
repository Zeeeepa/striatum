package reads

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

func TestLaneTrajectoryRowsRedactSecretsAndKeepStableFields(t *testing.T) {
	token := strings.Repeat("b", 64)
	rows := laneTrajectoryRowsFromRecords("run_1", "curated", []map[string]any{
		{
			"kind":       "agent_message",
			"ts":         "2026-06-14T00:00:00Z",
			"session_id": "sess_1",
			"role_id":    "implementer",
			"lane_id":    "codex",
			"body": map[string]any{
				"text":         "do not leak " + token,
				"message_kind": "note",
				"path":         ".striatum/transcripts/raw.log",
				"unsafe":       "private prose",
			},
		},
	})

	body, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	encoded := string(body)
	if strings.Contains(encoded, token) || strings.Contains(encoded, "private prose") {
		t.Fatalf("lane trajectory leaked secret material: %s", encoded)
	}
	if !strings.Contains(rows[0]["text"].(string), "<redacted-token>") {
		t.Fatalf("lane trajectory did not mark token redaction: %#v", rows[0])
	}
	eventFields := rows[0]["event_fields"].(map[string]any)
	if eventFields["path"] != evidenceFreeTextPlaceholder {
		t.Fatalf("lane trajectory did not mark redactions: %s", encoded)
	}
	if rows[0]["source_class"] != "lane_trajectory" || rows[0]["redaction_tier"] != "curated" {
		t.Fatalf("row metadata = %#v", rows[0])
	}
}

type laneTrajectoryCorpusRunner struct {
	token string
}

func (r laneTrajectoryCorpusRunner) Exec(context.Context, string, ...any) error {
	return errors.New("lane trajectory corpus must be read-only")
}

func (r laneTrajectoryCorpusRunner) QueryRow(context.Context, string, ...any) db.Row {
	return dashboardAllFakeRow{}
}

func (r laneTrajectoryCorpusRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "false", nil
}

func (r laneTrajectoryCorpusRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("lane trajectory corpus must not open a transaction")
}

func (r laneTrajectoryCorpusRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.runs"):
		return dashboardAllRowsFromMaps([]map[string]any{{"run_id": "run_b"}, {"run_id": "run_a"}}), nil
	case strings.Contains(sql, "ROW_NUMBER() OVER"):
		runID := args[1].(string)
		return dashboardAllRowsFromMaps([]map[string]any{
			{
				"kind":       "agent_message",
				"ts":         "2026-06-14T00:00:00Z",
				"session_id": "sess_" + runID,
				"role_id":    "implementer",
				"lane_id":    "codex",
				"body": map[string]any{
					"kind": "note",
					"body": map[string]any{"text": "use prior artifact " + r.token},
				},
			},
		}), nil
	default:
		return nil, errors.New("unexpected lane trajectory query")
	}
}

func TestLaneTrajectoryCorpusIsDeterministicHashedAndRedacted(t *testing.T) {
	token := strings.Repeat("c", 64)
	runner := laneTrajectoryCorpusRunner{token: token}

	first, err := laneTrajectoryCorpus(context.Background(), runner, "repo_1", "", "curated", 2)
	if err != nil {
		t.Fatalf("laneTrajectoryCorpus first: %v", err)
	}
	second, err := laneTrajectoryCorpus(context.Background(), runner, "repo_1", "", "curated", 2)
	if err != nil {
		t.Fatalf("laneTrajectoryCorpus second: %v", err)
	}

	if first["sha256"] == "" || first["sha256"] != second["sha256"] {
		t.Fatalf("unstable hash: first=%#v second=%#v", first["sha256"], second["sha256"])
	}
	body, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal corpus: %v", err)
	}
	encoded := string(body)
	if strings.Contains(encoded, token) || !strings.Contains(encoded, "redacted-token") {
		t.Fatalf("corpus redaction/hash payload = %s", encoded)
	}
	if first["row_count"] != 2 {
		t.Fatalf("row_count = %#v", first["row_count"])
	}
}
