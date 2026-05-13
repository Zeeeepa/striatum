package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func CanonicalHash(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func V2RowHash(row map[string]any) (string, error) {
	material := map[string]any{
		"ts":                  row["ts"],
		"schema_version":      row["schema_version"],
		"hash_format_version": row["hash_format_version"],
		"daemon_version":      row["daemon_version"],
		"client_id":           row["client_id"],
		"repository_id":       row["repository_id"],
		"method":              row["method"],
		"decision":            row["decision"],
		"denial_reason":       row["denial_reason"],
		"transport":           row["transport"],
		"request_id":          row["request_id"],
		"exit_code":           row["exit_code"],
		"params_sha256":       row["params_sha256"],
		"previous_hash":       row["previous_hash"],
		"segment_id":          row["segment_id"],
	}
	return CanonicalHash(material)
}

type AuditRecorder struct {
	Runner        Runner
	DaemonVersion string
}

func (a AuditRecorder) RecordRPC(ctx context.Context, envelope rpc.Envelope, auth rpc.AuthContext, response rpc.Response) (string, error) {
	if a.Runner == nil {
		return "", nil
	}
	paramsHash, err := CanonicalHash(envelope.Params)
	if err != nil {
		return "", err
	}
	previousHash, _ := a.Runner.QueryScalar(ctx, "SELECT last_hash FROM striatumd.audit_chain_head WHERE singleton = true")
	segmentID, _ := a.Runner.QueryScalar(ctx, "SELECT segment_id FROM striatumd.audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1")
	if segmentID == "" {
		segmentID = "1"
	}
	exitCode := "NULL"
	if !response.OK {
		exitCode = "10"
	}
	row := map[string]any{
		"ts":                  time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		"schema_version":      1,
		"hash_format_version": 2,
		"daemon_version":      a.DaemonVersion,
		"client_id":           nullString(auth.ClientID),
		"repository_id":       nullString(auth.RepositoryID),
		"method":              envelope.Method,
		"decision":            auth.Decision,
		"denial_reason":       nullString(auth.DenialReason),
		"transport":           "rpc",
		"request_id":          envelope.RequestID,
		"exit_code":           exitCode,
		"params_sha256":       paramsHash,
		"previous_hash":       nullString(previousHash),
		"segment_id":          segmentID,
	}
	rowHash, err := V2RowHash(row)
	if err != nil {
		return "", err
	}
	sql := fmt.Sprintf(`
WITH inserted AS (
  INSERT INTO striatumd.audit_log (
    ts, schema_version, hash_format_version, daemon_version,
    client_id, repository_id, method, decision, denial_reason,
    transport, request_id, exit_code, params_sha256, previous_hash,
    row_hash, segment_id
  )
  VALUES (
    %s, 1, 2, %s, %s, %s, %s, %s, %s, 'rpc', %s, %s, %s, %s, %s, %s
  )
  RETURNING audit_id, row_hash
)
UPDATE striatumd.audit_chain_head
SET last_audit_id = inserted.audit_id, last_hash = inserted.row_hash, updated_at = now()
FROM inserted
WHERE singleton = true;`,
		quoteLiteral(row["ts"].(string)),
		quoteLiteral(a.DaemonVersion),
		sqlNullable(row["client_id"]),
		sqlNullable(row["repository_id"]),
		quoteLiteral(envelope.Method),
		quoteLiteral(auth.Decision),
		sqlNullable(row["denial_reason"]),
		quoteLiteral(envelope.RequestID),
		exitCode,
		quoteLiteral(paramsHash),
		sqlNullable(row["previous_hash"]),
		quoteLiteral(rowHash),
		segmentID,
	)
	if err := a.Runner.Exec(ctx, sql); err != nil {
		return "", err
	}
	return "", nil
}

func VerifyRows(rows []map[string]any) []map[string]any {
	problems := []map[string]any{}
	var previous any
	for _, row := range rows {
		if row["previous_hash"] != previous {
			problems = append(problems, map[string]any{
				"check":   "daemon_pg_audit_chain",
				"id":      fmt.Sprint(row["audit_id"]),
				"message": "daemon PostgreSQL audit hash chain is broken",
				"context": map[string]any{"expected_previous_hash": previous, "actual_previous_hash": row["previous_hash"]},
			})
			return problems
		}
		computed, err := V2RowHash(row)
		if err != nil || computed != fmt.Sprint(row["row_hash"]) {
			problems = append(problems, map[string]any{
				"check":   "daemon_pg_audit_row_hash",
				"id":      fmt.Sprint(row["audit_id"]),
				"message": "daemon PostgreSQL audit row hash is invalid",
			})
			return problems
		}
		previous = row["row_hash"]
	}
	return problems
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func sqlNullable(value any) string {
	if value == nil {
		return "NULL"
	}
	return quoteLiteral(fmt.Sprint(value))
}
