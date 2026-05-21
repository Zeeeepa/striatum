package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// RecordRPC appends a single audit row inside a transaction that locks the
// singleton audit_chain_head row for the duration of the append. The
// hash-chain link cannot diverge under concurrent RPC traffic because the
// row-level lock serializes the read-then-write on the contended row.
func (a AuditRecorder) RecordRPC(
	ctx context.Context,
	envelope rpc.Envelope,
	auth rpc.AuthContext,
	response rpc.Response,
) (string, error) {
	return a.RecordRPCTransport(ctx, envelope, auth, response, "rpc")
}

func (a AuditRecorder) RecordRPCTransport(
	ctx context.Context,
	envelope rpc.Envelope,
	auth rpc.AuthContext,
	response rpc.Response,
	transport string,
) (string, error) {
	if a.Runner == nil {
		return "", nil
	}
	if transport == "" {
		transport = "rpc"
	}
	paramsHash, err := CanonicalHash(envelope.Params)
	if err != nil {
		return "", err
	}

	tx, err := a.Runner.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	// FOR UPDATE locks the singleton chain head row so the previous_hash we
	// observe is the one we will overwrite, end-to-end, in this transaction.
	var lastHash *string
	if err := tx.QueryRow(
		ctx,
		"SELECT last_hash FROM striatumd.audit_chain_head WHERE singleton = true FOR UPDATE",
	).Scan(&lastHash); err != nil {
		return "", fmt.Errorf("lock audit_chain_head: %w", err)
	}

	segmentID, err := tx.QueryScalar(
		ctx,
		"SELECT segment_id FROM striatumd.audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1",
	)
	if err != nil {
		return "", err
	}
	if segmentID == "" {
		if err := tx.Exec(
			ctx,
			"INSERT INTO striatumd.audit_segments(opened_at, state, retention_state) VALUES (now(), 'open', 'active')",
		); err != nil {
			return "", err
		}
		segmentID, err = tx.QueryScalar(
			ctx,
			"SELECT segment_id FROM striatumd.audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1",
		)
		if err != nil {
			return "", err
		}
		if segmentID == "" {
			return "", errors.New("daemon audit segment could not be created")
		}
	}
	segmentInt, err := strconv.ParseInt(segmentID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse segment id %q: %w", segmentID, err)
	}

	var exitCodeValue any
	var exitCodeForHash any
	if !response.OK {
		exitCodeValue = 10
		exitCodeForHash = 10
	}

	tsString := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	row := map[string]any{
		"ts":                  tsString,
		"schema_version":      1,
		"hash_format_version": 2,
		"daemon_version":      a.DaemonVersion,
		"client_id":           nullString(auth.ClientID),
		"repository_id":       nullString(auth.RepositoryID),
		"method":              envelope.Method,
		"decision":            auth.Decision,
		"denial_reason":       nullString(auth.DenialReason),
		"transport":           transport,
		"request_id":          envelope.RequestID,
		"exit_code":           exitCodeForHash,
		"params_sha256":       paramsHash,
		"previous_hash":       nullableFromPtr(lastHash),
		"segment_id":          segmentInt,
	}
	rowHash, err := V2RowHash(row)
	if err != nil {
		return "", err
	}

	var auditID int64
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO striatumd.audit_log (
			ts, schema_version, hash_format_version, daemon_version,
			client_id, repository_id, method, decision, denial_reason,
			transport, request_id, exit_code, params_sha256, previous_hash,
			row_hash, segment_id
		) VALUES (
			$1, 1, 2, $2,
			$3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14
		) RETURNING audit_id`,
		tsString,
		a.DaemonVersion,
		nullString(auth.ClientID),
		nullString(auth.RepositoryID),
		envelope.Method,
		auth.Decision,
		nullString(auth.DenialReason),
		transport,
		envelope.RequestID,
		exitCodeValue,
		paramsHash,
		nullableFromPtr(lastHash),
		rowHash,
		segmentInt,
	).Scan(&auditID); err != nil {
		return "", fmt.Errorf("insert audit row: %w", err)
	}

	if err := tx.Exec(
		ctx,
		`UPDATE striatumd.audit_chain_head
		 SET last_audit_id = $1, last_hash = $2, updated_at = now()
		 WHERE singleton = true`,
		auditID,
		rowHash,
	); err != nil {
		return "", fmt.Errorf("update audit_chain_head: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	committed = true
	return strconv.FormatInt(auditID, 10), nil
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

func nullableFromPtr(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
