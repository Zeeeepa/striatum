package reads

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleWorkPacketShow returns read-only packet audit metadata. The raw packet
// body is intentionally opt-in because work packet JSON carries full task prose.
func HandleWorkPacketShow(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	selectors := map[string]string{
		"packet_id":  stringParam(envelope, "packet_id"),
		"message_id": stringParam(envelope, "message_id"),
		"job_id":     stringParam(envelope, "job_id"),
		"session_id": stringParam(envelope, "session_id"),
		"run_id":     stringParam(envelope, "run_id"),
	}
	where := []string{"repository_id = $1"}
	args := []any{repositoryID}
	for _, key := range []string{"packet_id", "message_id", "job_id", "session_id", "run_id"} {
		value := selectors[key]
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		args = append(args, value)
		where = append(where, key+" = $"+strconv.Itoa(len(args)))
	}
	if len(args) == 1 {
		return nil, rpc.NewError("schema_invalid", "work.packet_show requires one of packet_id, message_id, job_id, session_id, or run_id", nil)
	}
	limit := packetAuditLimit(envelope)
	args = append(args, limit)
	raw := packetAuditRaw(envelope)
	selectList := `packet_id, run_id, job_id, message_id, lease_id, session_id, packet_sha256, created_at`
	if raw {
		selectList += `, packet_json`
	}
	rows, err := collectRows(ctx, runner,
		fmt.Sprintf(`SELECT %s
		   FROM striatumd.work_packets
		  WHERE %s
		  ORDER BY created_at DESC, packet_id DESC
		  LIMIT $%d`, selectList, strings.Join(where, " AND "), len(args)),
		args...,
	)
	if err != nil {
		return nil, err
	}
	if !raw {
		for _, row := range rows {
			delete(row, "packet_json")
		}
	}
	return map[string]any{
		"count":               len(rows),
		"packets":             rows,
		"raw_packet_included": raw,
		"limit":               limit,
	}, nil
}

func packetAuditRaw(envelope rpc.Envelope) bool {
	for _, key := range []string{"raw", "include_raw", "include_packet_json"} {
		if value, ok := envelope.Params[key].(bool); ok && value {
			return true
		}
	}
	return false
}

func packetAuditLimit(envelope rpc.Envelope) int {
	raw, ok := envelope.Params["limit"]
	if !ok || raw == nil {
		return 20
	}
	value, ok := numberParam(raw)
	if !ok || value <= 0 {
		return 20
	}
	if value > 200 {
		return 200
	}
	return value
}
