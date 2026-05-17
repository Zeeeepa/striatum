package dogfood

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	maxReasonLength  = 1000
	minExtendSeconds = 60
	maxExtendSeconds = 3600
)

var allowedVerdicts = map[string]bool{
	"accept":               true,
	"accept_with_findings": true,
	"needs_revision":       true,
	"reject":               true,
}

// Service owns the Go daemon dogfood composite surface.
//
// The legacy Python implementations live in striatum.dogfood.operator_tools and
// mutate a repo-local SQLite connection. Until those composites are ported to
// the Postgres-native Go mutation helpers end-to-end, the production Go daemon
// registers explicit fail-closed handlers so callers never fall through to a
// generic not_implemented stub or a Python/SQLite dependency.
type Service struct {
	Runner db.Runner
}

func (s Service) Register(server *rpc.Server) {
	if s.Runner == nil {
		return
	}
	server.Register("dogfood.publish_on_behalf", s.HandlePublishOnBehalf)
	server.Register("dogfood.surgical_recovery", s.HandleSurgicalRecovery)
}

func (s Service) HandlePublishOnBehalf(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := repositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if err := requireStringParams(envelope, "session_id", "artifact_path", "artifact_kind", "logical_name"); err != nil {
		return nil, err
	}
	reason, err := validatedReason(envelope)
	if err != nil {
		return nil, err
	}
	if verdict := optionalString(envelope, "verdict"); verdict != "" && !allowedVerdicts[verdict] {
		return nil, rpc.NewError("invalid_verdict", fmt.Sprintf("unknown verdict %q", verdict), nil)
	}
	return nil, rpc.NewError(
		"dogfood_publish_on_behalf_retired",
		"dogfood.publish_on_behalf is not available in the Go production daemon because the legacy composite is SQLite-bound; use daemon-native work.ack, artifact.publish, review.verdict, and work.complete until the Postgres composite is ported",
		map[string]any{
			"operation":     "dogfood.publish_on_behalf",
			"repository_id": repositoryID,
			"reason":        reason,
			"blocker":       "legacy_python_composite_uses_repo_local_sqlite_connection",
			"remediation": []string{
				"work.ack",
				"artifact.publish",
				"review.verdict or work.complete",
			},
		},
	)
}

func (s Service) HandleSurgicalRecovery(_ context.Context, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := repositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if err := requireStringParams(envelope, "job_id"); err != nil {
		return nil, err
	}
	reason, err := validatedReason(envelope)
	if err != nil {
		return nil, err
	}
	extendSeconds, err := intParam(envelope, "extend_lease_seconds", 900)
	if err != nil {
		return nil, err
	}
	if extendSeconds < minExtendSeconds || extendSeconds > maxExtendSeconds {
		return nil, rpc.NewError(
			"invalid_lease_extension",
			fmt.Sprintf("extend_lease_seconds must be between %d and %d", minExtendSeconds, maxExtendSeconds),
			map[string]any{"extend_lease_seconds": extendSeconds},
		)
	}
	if !boolParam(envelope, "confirm_write") {
		return nil, rpc.NewError(
			"confirm_write_required",
			"dogfood.surgical_recovery requires confirm_write=true because it bypasses ordinary stale-lease recovery policy",
			map[string]any{"operation": "dogfood.surgical_recovery"},
		)
	}
	return nil, rpc.NewError(
		"dogfood_surgical_recovery_retired",
		"dogfood.surgical_recovery is not available in the Go production daemon because the legacy composite is SQLite-bound and depends on dogfood-only operator recovery policy; use ordinary recovery methods or port the Postgres row-lock composite before enabling it",
		map[string]any{
			"operation":            "dogfood.surgical_recovery",
			"repository_id":        repositoryID,
			"job_id":               optionalString(envelope, "job_id"),
			"reason":               reason,
			"extend_lease_seconds": extendSeconds,
			"blocker":              "legacy_python_composite_uses_repo_local_sqlite_connection_and_progress_lock_policy",
			"remediation": []string{
				"recovery.stale_leases",
				"recovery.requeue_stale for non-repo-write work",
				"recovery.cancel_job after operator inspection",
			},
		},
	)
}

func repositoryID(envelope rpc.Envelope) (string, error) {
	value := optionalString(envelope, "repository_id")
	if value == "" {
		return "", rpc.NewError("repo_not_registered", "daemon RPC route requires repository_id", nil)
	}
	return value, nil
}

func requireStringParams(envelope rpc.Envelope, keys ...string) error {
	missing := []string{}
	for _, key := range keys {
		if optionalString(envelope, key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return rpc.NewError(
		"schema_invalid",
		"dogfood composite requires "+strings.Join(missing, ", "),
		map[string]any{"missing": missing},
	)
}

func validatedReason(envelope rpc.Envelope) (string, error) {
	reason := strings.TrimSpace(optionalString(envelope, "reason"))
	if reason == "" {
		return "", rpc.NewError("reason_required", "operator reason must not be empty", nil)
	}
	if len(reason) > maxReasonLength {
		return "", rpc.NewError(
			"reason_too_long",
			fmt.Sprintf("operator reason must be at most %d characters", maxReasonLength),
			map[string]any{"max_reason_length": maxReasonLength},
		)
	}
	return reason, nil
}

func optionalString(envelope rpc.Envelope, key string) string {
	value, _ := envelope.Params[key].(string)
	return strings.TrimSpace(value)
}

func boolParam(envelope rpc.Envelope, key string) bool {
	value, _ := envelope.Params[key].(bool)
	return value
}

func intParam(envelope rpc.Envelope, key string, fallback int) (int, error) {
	value, ok := envelope.Params[key]
	if !ok || value == nil {
		return fallback, nil
	}
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), nil
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), nil
		}
	}
	return 0, rpc.NewError("schema_invalid", key+" must be an integer", map[string]any{"field": key})
}
