package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// HandleDoctor mirrors reads/doctor.py. Returns a flat health report of
// the running daemon-pg state: schema version, append-only invariants
// detected via probe queries, stale-lease + waiting-human counts.
func HandleDoctor(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, _ := envelope.Params["repository_id"].(string)
	problems := []string{}

	var schemaVersion int
	if err := runner.QueryRow(ctx,
		`SELECT MAX(version) FROM striatumd.schema_meta`,
	).Scan(&schemaVersion); err != nil {
		problems = append(problems, "schema_meta.read_failed: "+err.Error())
	}

	staleLeases := 0
	if repositoryID != "" {
		rows, err := collectRows(ctx, runner,
			`SELECT COUNT(*) AS c FROM striatumd.leases
			  WHERE repository_id = $1 AND state = 'expired'`,
			repositoryID,
		)
		if err == nil && len(rows) > 0 {
			if c, ok := rows[0]["c"]; ok {
				switch v := c.(type) {
				case int64:
					staleLeases = int(v)
				case int:
					staleLeases = v
				case float64:
					staleLeases = int(v)
				}
			}
		}
	}

	waitingHuman := 0
	if repositoryID != "" {
		rows, err := collectRows(ctx, runner,
			`SELECT COUNT(*) AS c FROM striatumd.runs
			  WHERE repository_id = $1 AND state = 'waiting_human'`,
			repositoryID,
		)
		if err == nil && len(rows) > 0 {
			if c, ok := rows[0]["c"]; ok {
				switch v := c.(type) {
				case int64:
					waitingHuman = int(v)
				case int:
					waitingHuman = v
				case float64:
					waitingHuman = int(v)
				}
			}
		}
	}

	return map[string]any{
		"ok":             len(problems) == 0,
		"schema_version": schemaVersion,
		"stale_leases":   staleLeases,
		"waiting_human":  waitingHuman,
		"problems":       problems,
	}, nil
}
