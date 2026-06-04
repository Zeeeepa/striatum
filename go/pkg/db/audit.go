package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

// sqlStateInvalidAuthorization is SQLSTATE 28000, raised by
// assert_daemon_authority when the presented secret is missing or no longer
// matches a registry digest (RFC 0110 §4.5).
const sqlStateInvalidAuthorization = "28000"

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

// V3RowHash computes the RFC 0110 §5.1 v3 audit row hash: a length-prefixed
// canonical byte stream over the 15 fields in fixed declared order, escaping-
// free and key-order-free, so it is byte-identical to the in-database
// striatumd.audit_v3_row_hash builder (T-HASH-PARITY). V2RowHash is preserved
// permanently as the reader of pre-cutover (v2) rows.
func V3RowHash(row map[string]any) (string, error) {
	var buf bytes.Buffer
	v3encText(&buf, row["ts"])
	v3encInt(&buf, row["schema_version"])
	v3encInt(&buf, row["hash_format_version"])
	v3encText(&buf, row["daemon_version"])
	v3encText(&buf, row["client_id"])
	v3encText(&buf, row["repository_id"])
	v3encText(&buf, row["method"])
	v3encText(&buf, row["decision"])
	v3encText(&buf, row["denial_reason"])
	v3encText(&buf, row["transport"])
	v3encText(&buf, row["request_id"])
	v3encInt(&buf, row["exit_code"])
	v3encText(&buf, row["params_sha256"])
	v3encText(&buf, row["previous_hash"])
	v3encInt(&buf, row["segment_id"])
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

// v3encText writes the v3 encoding of a text (or null) field: a null field is
// the 3 bytes "-1:"; a present string s is octet_length(utf8(s)) as decimal
// ASCII, then ':', then the UTF-8 bytes.
func v3encText(buf *bytes.Buffer, value any) {
	if value == nil {
		buf.WriteString("-1:")
		return
	}
	var s string
	switch v := value.(type) {
	case string:
		s = v
	default:
		s = fmt.Sprint(v)
	}
	b := []byte(s)
	buf.WriteString(strconv.Itoa(len(b)))
	buf.WriteByte(':')
	buf.Write(b)
}

// v3encInt writes the v3 encoding of an integer (or null) field: the integer is
// rendered to decimal ASCII and then length-prefixed exactly like a string.
func v3encInt(buf *bytes.Buffer, value any) {
	if value == nil {
		buf.WriteString("-1:")
		return
	}
	var s string
	switch n := value.(type) {
	case int:
		s = strconv.Itoa(n)
	case int32:
		s = strconv.FormatInt(int64(n), 10)
	case int64:
		s = strconv.FormatInt(n, 10)
	default:
		s = fmt.Sprint(n)
	}
	b := []byte(s)
	buf.WriteString(strconv.Itoa(len(b)))
	buf.WriteByte(':')
	buf.Write(b)
}

// rowHashForFormat dispatches a row to its hash builder by hash_format_version:
// 2 -> V2RowHash, 3 -> V3RowHash, anything else -> error (never a silent v2
// fallback), so the verifier fails closed across an unknown format.
func rowHashForFormat(row map[string]any) (string, error) {
	switch fmt.Sprint(row["hash_format_version"]) {
	case "2":
		return V2RowHash(row)
	case "3":
		return V3RowHash(row)
	default:
		return "", fmt.Errorf("unknown audit hash_format_version %v", row["hash_format_version"])
	}
}

type AuditRecorder struct {
	Runner        Runner
	DaemonVersion string
}

// AuditMeta is the content of one audit row, independent of how the enclosing
// transaction is opened. It is built once (from the RPC envelope/auth/response,
// or from the dispatch-threaded context) and then written by appendAuditRow,
// whether the row is mutation-coupled (RFC 0110 §4.4, inside the handler's
// transaction) or standalone (its own authorized transaction).
type AuditMeta struct {
	DaemonVersion string
	ClientID      string
	RepositoryID  string
	Method        string
	Decision      string
	DenialReason  string
	Transport     string
	RequestID     string
	ParamsSHA256  string
	// OK reflects the recorded outcome; a non-OK row carries exit_code 10.
	OK bool
}

// RecordRPC appends a single standalone audit row in its own authorized
// transaction that locks the singleton audit_chain_head row for the duration of
// the append. The hash-chain link cannot diverge under concurrent RPC traffic
// because the row-level lock serializes the read-then-write on the contended
// row.
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
	meta, err := buildAuditMeta(a.DaemonVersion, transport, envelope, auth, response.OK)
	if err != nil {
		return "", err
	}
	// RFC 0110 §4.4: the standalone audit append rides its own authorized
	// transaction, the same prelude every mutation gets.
	tx, err := BeginAuthorizedMutation(ctx, a.Runner, AuthorityFromContext(ctx))
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	auditID, err := appendAuditRow(ctx, tx, meta)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	committed = true
	return auditID, nil
}

// AppendAuditInTx appends one audit row inside an existing transaction — the
// RFC 0110 §4.4 mutation-coupled path. The row commits or rolls back atomically
// with the mutation it records, so success-without-audit-row is impossible by
// construction. Callers hold the transaction; this never commits or rolls back.
func AppendAuditInTx(ctx context.Context, tx TxRunner, meta AuditMeta) (string, error) {
	if AuditHashFormat() == AuditHashFormatV3 {
		return appendAuditRowV3(ctx, tx, meta)
	}
	return appendAuditRow(ctx, tx, meta)
}

// appendAuditRowV3 routes the append through the owner-owned SECURITY DEFINER
// striatumd.append_audit_row, which asserts daemon authority (against the secret
// the prelude already set in this transaction), computes the v3 hash in-DB, and
// chains the row — all atomic with the caller's mutation (RFC 0110 §4.4, §5.2).
// A lost/again-rotated authority surfaces as a structured daemon_auth_lost
// (§4.5) rather than a bare 28000.
func appendAuditRowV3(ctx context.Context, tx TxRunner, meta AuditMeta) (string, error) {
	auditID, err := tx.QueryScalar(
		ctx,
		`SELECT striatumd.append_audit_row($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		meta.DaemonVersion,
		nullString(meta.ClientID),
		nullString(meta.RepositoryID),
		meta.Method,
		meta.Decision,
		nullString(meta.DenialReason),
		meta.Transport,
		meta.RequestID,
		meta.OK,
		meta.ParamsSHA256,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == sqlStateInvalidAuthorization {
			return "", rpc.NewError(
				"daemon_auth_lost",
				"daemon authority is no longer valid (registry row missing/superseded — restart the daemon to re-bootstrap, or check for a concurrent rotator)",
				map[string]any{"sqlstate": sqlStateInvalidAuthorization},
			)
		}
		return "", fmt.Errorf("append_audit_row (v3): %w", err)
	}
	return auditID, nil
}

// buildAuditMeta assembles an AuditMeta from an RPC envelope/auth and an outcome.
func buildAuditMeta(daemonVersion, transport string, envelope rpc.Envelope, auth rpc.AuthContext, ok bool) (AuditMeta, error) {
	if transport == "" {
		transport = "rpc"
	}
	paramsHash, err := CanonicalHash(envelope.Params)
	if err != nil {
		return AuditMeta{}, err
	}
	return AuditMeta{
		DaemonVersion: daemonVersion,
		ClientID:      auth.ClientID,
		RepositoryID:  auth.RepositoryID,
		Method:        envelope.Method,
		Decision:      auth.Decision,
		DenialReason:  auth.DenialReason,
		Transport:     transport,
		RequestID:     envelope.RequestID,
		ParamsSHA256:  paramsHash,
		OK:            ok,
	}, nil
}

// BuildAuditMetaFromContext assembles the audit meta for a mutation-coupled
// append from the dispatch-threaded envelope, auth, and audit dispatch. ok
// reflects the outcome being recorded (true for a successful mutation about to
// commit). The second return is false when the context lacks the dispatch
// threading (e.g. a direct handler unit test), in which case the caller skips
// the in-transaction append and leaves auditing to the standalone path.
func BuildAuditMetaFromContext(ctx context.Context, ok bool) (AuditMeta, bool, error) {
	envelope, hasEnvelope := rpc.EnvelopeFromContext(ctx)
	dispatch, hasDispatch := rpc.AuditDispatchFromContext(ctx)
	if !hasEnvelope || !hasDispatch {
		return AuditMeta{}, false, nil
	}
	auth, _ := rpc.AuthFromContext(ctx)
	meta, err := buildAuditMeta(dispatch.DaemonVersion, dispatch.Transport, envelope, auth, ok)
	if err != nil {
		return AuditMeta{}, false, err
	}
	return meta, true, nil
}

// appendAuditRow performs the chain-locked append of one audit row inside the
// supplied transaction. It locks the singleton chain head FOR UPDATE so the
// previous_hash it reads is the one it overwrites end-to-end within tx.
func appendAuditRow(ctx context.Context, tx TxRunner, meta AuditMeta) (string, error) {
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
	if !meta.OK {
		exitCodeValue = 10
		exitCodeForHash = 10
	}

	tsString := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	row := map[string]any{
		"ts":                  tsString,
		"schema_version":      1,
		"hash_format_version": 2,
		"daemon_version":      meta.DaemonVersion,
		"client_id":           nullString(meta.ClientID),
		"repository_id":       nullString(meta.RepositoryID),
		"method":              meta.Method,
		"decision":            meta.Decision,
		"denial_reason":       nullString(meta.DenialReason),
		"transport":           meta.Transport,
		"request_id":          meta.RequestID,
		"exit_code":           exitCodeForHash,
		"params_sha256":       meta.ParamsSHA256,
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
		meta.DaemonVersion,
		nullString(meta.ClientID),
		nullString(meta.RepositoryID),
		meta.Method,
		meta.Decision,
		nullString(meta.DenialReason),
		meta.Transport,
		meta.RequestID,
		exitCodeValue,
		meta.ParamsSHA256,
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
		computed, err := rowHashForFormat(row)
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
