package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type packetShowFakeRunner struct {
	query string
	args  []any
}

func (r *packetShowFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("work.packet_show must be read-only")
}

func (r *packetShowFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return packetShowFakeRow{}
}

func (r *packetShowFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r *packetShowFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("work.packet_show must not open a transaction")
}

func (r *packetShowFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	row := map[string]any{
		"packet_id":     "wp_1",
		"run_id":        "run_1",
		"job_id":        "job_1",
		"message_id":    "msg_1",
		"lease_id":      "lease_1",
		"session_id":    "sess_1",
		"packet_sha256": "abc123",
		"created_at":    "2026-06-08T00:00:00Z",
	}
	if strings.Contains(sql, "packet_json") {
		row["packet_json"] = map[string]any{"task_prompt": "private"}
	}
	return dashboardAllRowsFromMaps([]map[string]any{row}), nil
}

type packetShowFakeRow struct{}

func (packetShowFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestHandleWorkPacketShowRequiresASelector(t *testing.T) {
	_, err := HandleWorkPacketShow(context.Background(), &packetShowFakeRunner{}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	if err == nil {
		t.Fatal("expected selector error")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("error = %v, want schema_invalid", err)
	}
}

func TestHandleWorkPacketShowReturnsMetadataWithoutRawByDefault(t *testing.T) {
	runner := &packetShowFakeRunner{}
	result, err := HandleWorkPacketShow(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "packet_id": "wp_1"},
	})
	if err != nil {
		t.Fatalf("HandleWorkPacketShow: %v", err)
	}
	if strings.Contains(runner.query, "packet_json") {
		t.Fatalf("metadata query selected packet_json: %s", runner.query)
	}
	packets := result["packets"].([]map[string]any)
	if _, ok := packets[0]["packet_json"]; ok {
		t.Fatalf("packet_json leaked without raw: %#v", packets[0])
	}
	if packets[0]["packet_sha256"] != "abc123" {
		t.Fatalf("packet metadata = %#v", packets[0])
	}
}

func TestHandleWorkPacketShowIncludesRawOnlyWhenRequested(t *testing.T) {
	runner := &packetShowFakeRunner{}
	result, err := HandleWorkPacketShow(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1", "raw": true, "limit": 500},
	})
	if err != nil {
		t.Fatalf("HandleWorkPacketShow raw: %v", err)
	}
	if !strings.Contains(runner.query, "packet_json") {
		t.Fatalf("raw query did not select packet_json: %s", runner.query)
	}
	if runner.args[len(runner.args)-1] != 200 {
		t.Fatalf("limit arg = %#v, want cap 200", runner.args[len(runner.args)-1])
	}
	packets := result["packets"].([]map[string]any)
	packetJSON, _ := packets[0]["packet_json"].(map[string]any)
	if packetJSON["task_prompt"] != "private" {
		t.Fatalf("packet_json missing in raw result: %#v", packets[0])
	}
	if result["raw_packet_included"] != true {
		t.Fatalf("raw_packet_included = %#v", result["raw_packet_included"])
	}
}
