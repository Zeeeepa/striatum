package reads

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type trajectoryFakeRunner struct{}

func (trajectoryFakeRunner) Exec(context.Context, string, ...any) error { return nil }
func (trajectoryFakeRunner) QueryRow(context.Context, string, ...any) db.Row { return nil }
func (trajectoryFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) { return "", nil }
func (trajectoryFakeRunner) BeginTx(context.Context) (db.TxRunner, error) { return nil, nil }

func (trajectoryFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM striatumd.queue_messages") {
		return dashboardAllRowsFromMaps([]map[string]any{
			{
				"seq":       int64(10),
				"ts":        time.Now(),
				"kind":      "agent_message",
				"session_id": "sess1",
				"role_id":   "author",
				"lane_id":   "lane1",
				"body": map[string]any{
					"text":                "hello",
					"provider_transcript": "SHOULD BE REMOVED",
				},
			},
		}), nil
	}
	return dashboardAllRowsFromMaps(nil), nil
}

func TestHandleTrajectoryExportD028(t *testing.T) {
	runner := trajectoryFakeRunner{}
	envelope := rpc.Envelope{
		Params: map[string]any{
			"repository_id": "repo1",
			"run_id":        "run1",
		},
	}
	resp, err := HandleTrajectoryExport(context.Background(), runner, envelope)
	if err != nil {
		t.Fatalf("HandleTrajectoryExport failed: %v", err)
	}
	records := resp["records"].([]map[string]any)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	body := records[0]["body"].(map[string]any)
	if _, ok := body["provider_transcript"]; ok {
		t.Errorf("D028 violation: provider_transcript should have been curated out")
	}
	if text := body["text"].(string); text != "hello" {
		t.Errorf("expected text 'hello', got '%s'", text)
	}
}
