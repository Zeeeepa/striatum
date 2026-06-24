package reads

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleWhose answers "which operator started this run" (RFC 0167 P0 D6 / §9.6).
// It is a pure identity join that CANNOT lie: it reads only through the
// daemon-gated run_origin_identity SECURITY DEFINER projection over the
// owner-held created_by_principal_id / created_by_handle_id columns — never
// tty/pane/title/env. The friendly handle#suffix is derived from the immutable
// principal_id and degrades to the bare id when no live handle resolves (a
// revoked/expired/never-leased origin), so the answer is never a confident lie.
func HandleWhose(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "whose requires run_id", nil)
	}
	secret := db.AuthorityFromContext(ctx).Secret
	rows, err := collectRows(ctx, runner,
		`SELECT run_id, state, created_by_principal_id, origin_handle, principal_kind, disabled_at
		   FROM striatumd.run_origin_identity($1, $2, $3)`,
		secret, repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
	}
	row := rows[0]
	principalID := stringValue(row["created_by_principal_id"])
	handle := stringValue(row["origin_handle"])
	return map[string]any{
		"run_id":         stringValue(row["run_id"]),
		"state":          stringValue(row["state"]),
		"principal_id":   principalID,
		"principal_kind": stringValue(row["principal_kind"]),
		"handle":         handle,
		"identity":       db.RenderOperatorIdentity(principalID, handle),
		"attributed":     principalID != "",
		"switch_hint":    fmt.Sprintf("striatum status --run-id %s", stringValue(row["run_id"])),
	}, nil
}
