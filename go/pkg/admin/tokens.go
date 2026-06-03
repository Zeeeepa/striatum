package admin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const defaultTokenTTL = time.Hour

type tokenGrant struct {
	Capability   rpc.Capability
	RepositoryID *string
	ExpiresAt    *time.Time
}

type tokenRecord struct {
	ClientID    string
	ClientKind  string
	DisplayName string
	TokenID     string
	TokenHash   string
	TokenSalt   string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
}

func (s Service) CreateToken(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "daemon.token.create requires daemon PostgreSQL", nil)
	}
	now := s.now()
	expiresAt, err := tokenExpiry(envelope.Params, now, nil)
	if err != nil {
		return nil, err
	}
	grants, err := grantsFromParams(envelope.Params, expiresAt)
	if err != nil {
		return nil, err
	}
	if err := validateGrantRepositories(ctx, s.Runner, grants); err != nil {
		return nil, err
	}
	clientKind := stringParam(envelope.Params, "client_kind")
	if clientKind == "" {
		clientKind = "cli"
	}
	displayName := stringParam(envelope.Params, "display_name")
	if displayName == "" {
		displayName = "admin-created"
	}
	tx, err := db.BeginAuthorizedMutation(ctx, s.Runner, db.AuthorityFromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(ctx, tx)
	result, err := insertTokenClient(ctx, tx, now, clientKind, displayName, grants, expiresAt)
	if err != nil {
		return nil, err
	}
	// RFC 0107: optionally attribute the minted client to a principal, in the
	// SAME transaction that issues the token, so a token always carries the
	// principal it belongs to (or none, for back-compat).
	if err := attributeMintedClient(ctx, tx, envelope.Params, result); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// attributeMintedClient links the just-minted client (whose id is in result)
// to the principal named in the request params, and reflects the principal id
// back in the result. A no-op when no principal_id was supplied.
func attributeMintedClient(ctx context.Context, tx db.TxRunner, params map[string]any, result map[string]any) error {
	principalID := stringParam(params, "principal_id")
	if principalID == "" {
		return nil
	}
	clientID, _ := result["client_id"].(string)
	if clientID == "" {
		return rpc.NewError("internal_error", "token issuance produced no client_id to attribute", nil)
	}
	if err := attributeClientToPrincipal(ctx, tx, params, clientID); err != nil {
		return err
	}
	result["principal_id"] = principalID
	return nil
}

func (s Service) RevokeToken(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "daemon.token.revoke requires daemon PostgreSQL", nil)
	}
	now := s.now()
	target, secret, hasSecret, err := tokenTarget(envelope.Params)
	if err != nil {
		return nil, err
	}
	reason := stringParam(envelope.Params, "reason")
	if reason == "" {
		reason = "operator_revoked"
	}
	tx, err := db.BeginAuthorizedMutation(ctx, s.Runner, db.AuthorityFromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(ctx, tx)
	record, err := loadTokenForUpdate(ctx, tx, target)
	if err != nil {
		return nil, err
	}
	if hasSecret && !secretMatches(record, secret) {
		return nil, rpc.NewError("token_invalid", "daemon token secret does not match token_id", nil)
	}
	alreadyRevoked := record.RevokedAt != nil
	capsRevoked, err := revokeTokenRows(ctx, tx, record.ClientID, now, reason)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"token_id":             record.TokenID,
		"client_id":            record.ClientID,
		"revoked":              true,
		"already_revoked":      alreadyRevoked,
		"revoked_at":           formatTime(now),
		"revoked_capabilities": capsRevoked,
	}, nil
}

func (s Service) RotateToken(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "daemon.token.rotate requires daemon PostgreSQL", nil)
	}
	now := s.now()
	target, secret, hasSecret, err := tokenTarget(envelope.Params)
	if err != nil {
		return nil, err
	}
	reason := stringParam(envelope.Params, "reason")
	if reason == "" {
		reason = "operator_rotated"
	}
	tx, err := db.BeginAuthorizedMutation(ctx, s.Runner, db.AuthorityFromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer rollbackQuietly(ctx, tx)
	record, err := loadTokenForUpdate(ctx, tx, target)
	if err != nil {
		return nil, err
	}
	if hasSecret && !secretMatches(record, secret) {
		return nil, rpc.NewError("token_invalid", "daemon token secret does not match token_id", nil)
	}
	if record.RevokedAt != nil {
		return nil, rpc.NewError("token_revoked", "daemon token is already revoked", nil)
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return nil, rpc.NewError("token_expired", "daemon token is expired", nil)
	}
	if ambiguous, err := activeScopeAmbiguity(ctx, tx, record.ClientID, now); err != nil {
		return nil, err
	} else if ambiguous != "" {
		return nil, rpc.NewError("token_scope_ambiguous", "daemon token has duplicate active capability scope", map[string]any{"scope": ambiguous})
	}
	grants, err := activeGrants(ctx, tx, record.ClientID, now)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return nil, rpc.NewError("capability_missing", "daemon token has no active capabilities to rotate", nil)
	}
	expiresAt, err := tokenExpiry(envelope.Params, now, record.ExpiresAt)
	if err != nil {
		return nil, err
	}
	for i := range grants {
		grants[i].ExpiresAt = minOptionalTime(grants[i].ExpiresAt, expiresAt)
	}
	capsRevoked, err := revokeTokenRows(ctx, tx, record.ClientID, now, reason)
	if err != nil {
		return nil, err
	}
	displayName := stringParam(envelope.Params, "display_name")
	if displayName == "" {
		displayName = record.DisplayName
	}
	clientKind := stringParam(envelope.Params, "client_kind")
	if clientKind == "" {
		clientKind = record.ClientKind
	}
	created, err := insertTokenClient(ctx, tx, now, clientKind, displayName, grants, expiresAt)
	if err != nil {
		return nil, err
	}
	// RFC 0107: carry the rotated-from client's principal onto the replacement
	// client (in the same transaction), so rotation keeps a stable principal
	// identity. An explicit principal_id param overrides the carry-over.
	principalID, err := carryPrincipalAcrossRotation(ctx, tx, envelope.Params, record.ClientID, created)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	created["rotated_from_token_id"] = record.TokenID
	created["rotated_from_client_id"] = record.ClientID
	created["revoked_capabilities"] = capsRevoked
	if principalID != "" {
		created["principal_id"] = principalID
	}
	return created, nil
}

// carryPrincipalAcrossRotation unlinks the rotated-from client from its
// principal and links the replacement client to the same principal, so the
// principal's identity survives token rotation. An explicit principal_id in the
// rotation params takes precedence (and is attributed via the usual create
// path). Returns the principal id the replacement client ends up attributed to,
// or "" when neither the old client nor the params named a principal.
func carryPrincipalAcrossRotation(ctx context.Context, tx db.TxRunner, params map[string]any, oldClientID string, created map[string]any) (string, error) {
	newClientID, _ := created["client_id"].(string)
	if newClientID == "" {
		return "", rpc.NewError("internal_error", "token rotation produced no client_id to attribute", nil)
	}
	explicit := stringParam(params, "principal_id")
	if explicit != "" {
		if err := attributeClientToPrincipal(ctx, tx, params, newClientID); err != nil {
			return "", err
		}
		// The old client is being revoked; drop its active attribution too.
		if err := unlinkClientFromPrincipals(ctx, tx, oldClientID); err != nil {
			return "", err
		}
		return explicit, nil
	}
	ref, ok, err := ResolvePrincipalForClient(ctx, tx, oldClientID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	if err := unlinkClientFromPrincipals(ctx, tx, oldClientID); err != nil {
		return "", err
	}
	if err := LinkClientToPrincipal(ctx, tx, ref.PrincipalID, newClientID); err != nil {
		return "", err
	}
	return ref.PrincipalID, nil
}

// unlinkClientFromPrincipals marks every active link for a client as unlinked.
func unlinkClientFromPrincipals(ctx context.Context, tx db.TxRunner, clientID string) error {
	return tx.Exec(ctx,
		`UPDATE striatumd.principal_clients
		    SET unlinked_at = now()
		  WHERE client_id = $1 AND unlinked_at IS NULL`,
		clientID,
	)
}

func (s Service) now() time.Time {
	return time.Now().UTC()
}

func insertTokenClient(ctx context.Context, tx db.TxRunner, now time.Time, clientKind string, displayName string, grants []tokenGrant, expiresAt *time.Time) (map[string]any, error) {
	tokenID := "dtok_" + randomURLSafe(12)
	secret := randomURLSafe(32)
	salt := randomHex(16)
	clientID := "dclient_" + randomHex(16)
	tokenHash := hmacHexToken(salt, secret)
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.clients(client_id, client_kind, display_name,
		  token_id, token_hash, token_salt, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		clientID, clientKind, displayName, tokenID, tokenHash, salt, now, nullableTime(expiresAt),
	); err != nil {
		return nil, err
	}
	for _, grant := range grants {
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.client_capabilities(
			  capability_id, client_id, repository_id, capability, granted_at, expires_at
			)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			"dcap_"+randomHex(16),
			clientID,
			nullableString(grant.RepositoryID),
			string(grant.Capability),
			now,
			nullableTime(grant.ExpiresAt),
		); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"token":        tokenID + "." + secret,
		"token_id":     tokenID,
		"client_id":    clientID,
		"client_kind":  clientKind,
		"display_name": displayName,
		"created_at":   formatTime(now),
		"expires_at":   formatOptionalTime(expiresAt),
		"capabilities": grantPayload(grants),
	}, nil
}

func grantsFromParams(params map[string]any, expiresAt *time.Time) ([]tokenGrant, error) {
	rawGrants, hasRawGrants := params["capability_grants"]
	if hasRawGrants {
		items, ok := rawGrants.([]any)
		if !ok || len(items) == 0 {
			return nil, rpc.NewError("schema_invalid", "daemon.token.create capability_grants must be a non-empty array", nil)
		}
		grants := make([]tokenGrant, 0, len(items))
		for _, item := range items {
			body, ok := item.(map[string]any)
			if !ok {
				return nil, rpc.NewError("schema_invalid", "daemon.token.create capability_grants entries must be objects", nil)
			}
			capability, err := parseCapability(body["capability"])
			if err != nil {
				return nil, err
			}
			repo := optionalString(body["repository_id"])
			if repo == nil {
				repo = optionalString(body["repo_id"])
			}
			grants = append(grants, tokenGrant{Capability: capability, RepositoryID: repo, ExpiresAt: expiresAt})
		}
		return dedupeGrants(grants)
	}
	capabilities, err := capabilitiesFromParams(params)
	if err != nil {
		return nil, err
	}
	repositoryID := optionalString(params["repository_id"])
	if repositoryID == nil {
		repositoryID = optionalString(params["repo_id"])
	}
	grants := make([]tokenGrant, 0, len(capabilities))
	for _, capability := range capabilities {
		grants = append(grants, tokenGrant{Capability: capability, RepositoryID: repositoryID, ExpiresAt: expiresAt})
	}
	return dedupeGrants(grants)
}

func capabilitiesFromParams(params map[string]any) ([]rpc.Capability, error) {
	if value, ok := params["capability"]; ok {
		capability, err := parseCapability(value)
		if err != nil {
			return nil, err
		}
		return []rpc.Capability{capability}, nil
	}
	value, ok := params["capabilities"]
	if !ok {
		return nil, rpc.NewError("schema_invalid", "daemon.token.create requires capabilities", nil)
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, rpc.NewError("schema_invalid", "daemon.token.create capabilities must be a non-empty array", nil)
	}
	capabilities := make([]rpc.Capability, 0, len(items))
	for _, item := range items {
		capability, err := parseCapability(item)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities, nil
}

func parseCapability(value any) (rpc.Capability, error) {
	name, ok := value.(string)
	if !ok || name == "" {
		return "", rpc.NewError("schema_invalid", "daemon token capability must be a non-empty string", nil)
	}
	capability := rpc.Capability(name)
	if _, ok := rpc.Capabilities[capability]; !ok {
		return "", rpc.NewError("schema_invalid", fmt.Sprintf("unsupported daemon capability %q", name), nil)
	}
	return capability, nil
}

func dedupeGrants(grants []tokenGrant) ([]tokenGrant, error) {
	seen := map[string]struct{}{}
	for _, grant := range grants {
		key := string(grant.Capability) + "\x00"
		if grant.RepositoryID != nil {
			key += *grant.RepositoryID
		}
		if _, ok := seen[key]; ok {
			return nil, rpc.NewError("schema_invalid", "daemon token capabilities contain a duplicate grant", nil)
		}
		seen[key] = struct{}{}
	}
	return grants, nil
}

func validateGrantRepositories(ctx context.Context, runner db.Runner, grants []tokenGrant) error {
	seen := map[string]struct{}{}
	for _, grant := range grants {
		if grant.RepositoryID == nil {
			continue
		}
		repoID := *grant.RepositoryID
		if repoID == "" {
			return rpc.NewError("schema_invalid", "daemon token repository_id must be non-empty when present", nil)
		}
		if _, ok := seen[repoID]; ok {
			continue
		}
		seen[repoID] = struct{}{}
		found, err := runner.QueryScalar(ctx, `
			SELECT repository_id
			  FROM striatumd.repositories
			 WHERE repository_id = $1 AND state != 'removed'`,
			repoID,
		)
		if err != nil {
			return err
		}
		if found == "" {
			return rpc.NewError("not_found", fmt.Sprintf("unknown daemon repository %q", repoID), nil)
		}
	}
	return nil
}

func tokenExpiry(params map[string]any, now time.Time, oldExpiry *time.Time) (*time.Time, error) {
	if value, ok := params["expires_at"]; ok && value != nil {
		text, ok := value.(string)
		if !ok || text == "" {
			return nil, rpc.NewError("schema_invalid", "daemon token expires_at must be an RFC3339 string", nil)
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return nil, rpc.NewError("schema_invalid", "daemon token expires_at must be an RFC3339 string", nil)
		}
		parsed = parsed.UTC()
		if !parsed.After(now) {
			return nil, rpc.NewError("schema_invalid", "daemon token expires_at must be in the future", nil)
		}
		return &parsed, nil
	}
	if value, ok := params["expires_in"]; ok && value != nil {
		return expiryFromDuration(value, now)
	}
	if value, ok := params["expires_in_seconds"]; ok && value != nil {
		return expiryFromDuration(value, now)
	}
	expiresAt := now.Add(defaultTokenTTL)
	if oldExpiry != nil && oldExpiry.After(now) && oldExpiry.Before(expiresAt) {
		expiresAt = *oldExpiry
	}
	return &expiresAt, nil
}

func expiryFromDuration(value any, now time.Time) (*time.Time, error) {
	duration, err := parseDurationValue(value)
	if err != nil {
		return nil, err
	}
	if duration <= 0 {
		return nil, rpc.NewError("schema_invalid", "daemon token expires_in must be positive", nil)
	}
	expiresAt := now.Add(duration)
	return &expiresAt, nil
}

func parseDurationValue(value any) (time.Duration, error) {
	switch v := value.(type) {
	case int:
		return time.Duration(v) * time.Second, nil
	case int64:
		return time.Duration(v) * time.Second, nil
	case float64:
		if math.Trunc(v) != v {
			return 0, rpc.NewError("schema_invalid", "daemon token expires_in must be whole seconds or a duration string", nil)
		}
		return time.Duration(v) * time.Second, nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, rpc.NewError("schema_invalid", "daemon token expires_in must be whole seconds or a duration string", nil)
		}
		return time.Duration(i) * time.Second, nil
	case string:
		if v == "" {
			return 0, rpc.NewError("schema_invalid", "daemon token expires_in must be non-empty", nil)
		}
		duration, err := time.ParseDuration(v)
		if err != nil {
			return 0, rpc.NewError("schema_invalid", "daemon token expires_in must be whole seconds or a duration string", nil)
		}
		return duration, nil
	default:
		return 0, rpc.NewError("schema_invalid", "daemon token expires_in must be whole seconds or a duration string", nil)
	}
}

func tokenTarget(params map[string]any) (string, string, bool, error) {
	tokenID := stringParam(params, "token_id")
	if tokenID == "" {
		tokenID = stringParam(params, "id")
	}
	token := stringParam(params, "token")
	if token != "" && tokenID != "" {
		return "", "", false, rpc.NewError("schema_invalid", "daemon token target must use token_id or token, not both", nil)
	}
	if token != "" {
		id, secret, ok := strings.Cut(token, ".")
		if !ok || id == "" || secret == "" {
			return "", "", false, rpc.NewError("token_malformed", "daemon token is malformed", nil)
		}
		return id, secret, true, nil
	}
	if tokenID == "" {
		return "", "", false, rpc.NewError("schema_invalid", "daemon token target requires token_id or token", nil)
	}
	return tokenID, "", false, nil
}

func loadTokenForUpdate(ctx context.Context, runner db.TxRunner, tokenID string) (tokenRecord, error) {
	var record tokenRecord
	var expiresAt, revokedAt pgtype.Timestamptz
	err := runner.QueryRow(ctx, `
		SELECT client_id, client_kind, display_name, token_id, token_hash,
		       token_salt, expires_at, revoked_at
		  FROM striatumd.clients
		 WHERE token_id = $1
		 FOR UPDATE`,
		tokenID,
	).Scan(
		&record.ClientID,
		&record.ClientKind,
		&record.DisplayName,
		&record.TokenID,
		&record.TokenHash,
		&record.TokenSalt,
		&expiresAt,
		&revokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tokenRecord{}, rpc.NewError("token_invalid", "daemon token does not exist", nil)
		}
		return tokenRecord{}, err
	}
	record.ExpiresAt = pgTimePtr(expiresAt)
	record.RevokedAt = pgTimePtr(revokedAt)
	return record, nil
}

func revokeTokenRows(ctx context.Context, tx db.TxRunner, clientID string, now time.Time, reason string) (int64, error) {
	var revoked int64
	if err := tx.QueryRow(ctx, `
		WITH revoked AS (
		  UPDATE striatumd.client_capabilities
		     SET revoked_at = $2, revoked_reason = $3
		   WHERE client_id = $1 AND revoked_at IS NULL
		   RETURNING 1
		)
		SELECT COUNT(*) FROM revoked`,
		clientID, now, reason,
	).Scan(&revoked); err != nil {
		return 0, err
	}
	if err := tx.Exec(ctx, `
		UPDATE striatumd.clients
		   SET revoked_at = COALESCE(revoked_at, $2)
		 WHERE client_id = $1`,
		clientID, now,
	); err != nil {
		return 0, err
	}
	return revoked, nil
}

func activeScopeAmbiguity(ctx context.Context, tx db.TxRunner, clientID string, now time.Time) (string, error) {
	value, err := tx.QueryScalar(ctx, `
		SELECT capability || ':' || COALESCE(repository_id, '<global>')
		  FROM striatumd.client_capabilities
		 WHERE client_id = $1
		   AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > $2)
		 GROUP BY capability, repository_id
		HAVING COUNT(*) > 1
		 LIMIT 1`,
		clientID, now,
	)
	if err != nil {
		return "", err
	}
	return value, nil
}

func activeGrants(ctx context.Context, tx db.TxRunner, clientID string, now time.Time) ([]tokenGrant, error) {
	payload, err := tx.QueryScalar(ctx, `
		SELECT COALESCE(
		  jsonb_agg(
		    jsonb_build_object(
		      'capability', capability,
		      'repository_id', repository_id,
		      'expires_at', expires_at
		    )
		    ORDER BY capability, repository_id NULLS FIRST
		  ),
		  '[]'::jsonb
		)::text
		  FROM striatumd.client_capabilities
		 WHERE client_id = $1
		   AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > $2)`,
		clientID, now,
	)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, err
	}
	grants := make([]tokenGrant, 0, len(raw))
	for _, item := range raw {
		capability, err := parseCapability(item["capability"])
		if err != nil {
			return nil, err
		}
		repo := optionalString(item["repository_id"])
		expiresAt, err := optionalTime(item["expires_at"])
		if err != nil {
			return nil, err
		}
		grants = append(grants, tokenGrant{Capability: capability, RepositoryID: repo, ExpiresAt: expiresAt})
	}
	return grants, nil
}

func secretMatches(record tokenRecord, secret string) bool {
	expected := hmacHexToken(record.TokenSalt, secret)
	return hmac.Equal([]byte(expected), []byte(record.TokenHash))
}

func hmacHexToken(salt string, secret string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomURLSafe(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func grantPayload(grants []tokenGrant) []map[string]any {
	payload := make([]map[string]any, 0, len(grants))
	for _, grant := range grants {
		payload = append(payload, map[string]any{
			"capability":    string(grant.Capability),
			"repository_id": nullableString(grant.RepositoryID),
			"expires_at":    formatOptionalTime(grant.ExpiresAt),
		})
	}
	return payload
}

func optionalString(value any) *string {
	if value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return nil
	}
	return &text
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func optionalTime(value any) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return nil, rpc.NewError("schema_invalid", "daemon token capability expires_at must be an RFC3339 string", nil)
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil, rpc.NewError("schema_invalid", "daemon token capability expires_at must be an RFC3339 string", nil)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func pgTimePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func minOptionalTime(a *time.Time, b *time.Time) *time.Time {
	if a == nil {
		return b
	}
	if b == nil || a.Before(*b) {
		return a
	}
	return b
}

func rollbackQuietly(ctx context.Context, tx db.TxRunner) {
	_ = tx.Rollback(ctx)
}
