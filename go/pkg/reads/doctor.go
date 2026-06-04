package reads

import (
	"context"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

const doctorSupervisorProbeTimeout = 5 * time.Second

// HandleDoctor mirrors reads/doctor.py. Returns a flat health report of
// the running daemon-pg state: schema version, append-only invariants
// detected via probe queries, stale-lease + waiting-human counts.
func HandleDoctor(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, _ := envelope.Params["repository_id"].(string)
	problems := []string{}

	schemaVersion, err := db.ReadSchemaVersion(ctx, runner)
	if err != nil {
		problems = append(problems, "schema_meta.read_failed: "+err.Error())
	}

	staleLeases := 0
	if repositoryID != "" {
		// #45: an expired lease row persists forever, so counting every
		// `state = 'expired'` lease keeps reporting stale leases long after the
		// work was recovered/completed. A lease is only genuinely stale when it
		// is still a job's CURRENT lease AND that job is still actionable
		// (claimed/running/stale_lease) — matching the authoritative predicate in
		// status.go. Recovery or completion swaps current_lease_id / advances the
		// job state, which makes the count drop as expected.
		rows, err := collectRows(ctx, runner,
			`SELECT COUNT(*) AS c
			   FROM striatumd.jobs j
			   JOIN striatumd.leases l
			     ON l.repository_id = j.repository_id
			    AND l.lease_id = j.current_lease_id
			  WHERE j.repository_id = $1
			    AND l.state = 'expired'
			    AND j.state IN ('claimed', 'running', 'stale_lease')`,
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
	// RFC 0101 Phase 4: a run flipped to needs_operator by the recovery sweep
	// (autonomous recovery exhausted its per-job budget) is a LOUD, actionable
	// problem — not a warning. Surface the count + the specific run ids and add
	// each to `problems` with the escalation reason so `ok` goes false.
	needsOperatorRuns := []string{}
	if repositoryID != "" {
		rows, err := collectRows(ctx, runner,
			`SELECT run_id FROM striatumd.runs
			  WHERE repository_id = $1 AND state = 'needs_operator'
			  ORDER BY run_id`,
			repositoryID,
		)
		if err == nil {
			for _, row := range rows {
				runID := superviseString(row["run_id"])
				if runID == "" {
					continue
				}
				needsOperatorRuns = append(needsOperatorRuns, runID)
				problems = append(problems, "run_needs_operator."+runID+": autonomous recovery exhausted; resolve the recovery_exhausted escalation (escalation resolve) or cancel the run")
			}
		}
	}

	supervisorLiveness := []map[string]any{}
	if repositoryID != "" {
		probeCtx, cancel := context.WithTimeout(ctx, doctorSupervisorProbeTimeout)
		defer cancel()
		if rows, err := reattachStatusRowsWithOptions(probeCtx, runner, repositoryID, "", "", "", reattachStatusRowsOptions{nonTerminalRunsOnly: true}); err == nil {
			for _, row := range rows {
				view := reattachStatusView(probeCtx, row)
				class := superviseString(view["lane_liveness_class"])
				if !strings.HasPrefix(class, "tmux_") {
					continue
				}
				deliveryLiveness := superviseObject(view["delivery_liveness"])
				deliveryClass := superviseString(deliveryLiveness["class"])
				deliveryReason := superviseString(deliveryLiveness["reason"])

				item := map[string]any{
					"supervisor_id": view["supervisor_id"],
					"session_id":    view["session_id"],
					"class":         class,
					"state":         view["reattach_state"],
					"reason":        view["reattach_reason"],
				}
				if deliveryClass == "degraded" {
					if remediation := deliveryRemediation(deliveryReason, superviseString(view["session_id"])); remediation != "" {
						item["remediation"] = remediation
					}
				} else if remediation := tmuxLivenessRemediation(class, superviseString(view["reattach_reason"]), superviseString(view["session_id"])); remediation != "" {
					item["remediation"] = remediation
				}
				supervisorLiveness = append(supervisorLiveness, item)
				if view["reattach_state"] != "terminal" && class != string(gosupervisor.TmuxLivenessOK) && class != string(gosupervisor.TmuxLivenessUnavailable) {
					problems = append(problems, "supervisor_liveness."+superviseString(view["supervisor_id"])+": "+class)
				}
				if deliveryClass == "degraded" {
					problems = append(problems, "supervisor_delivery_degraded."+superviseString(view["supervisor_id"])+": "+deliveryReason)
				}
			}
		}
	}

	// #64: advise (warn, never hard-fail) when ~/.codex/config.toml points at a
	// stale MCP endpoint or codex would start without a bearer. The token VALUE
	// is never read or returned.
	codexBlock, codexWarnings := codexDoctorBlock()
	warnings := append([]string{}, codexWarnings...)

	// #87 / RFC 0096 §2: surface (warn, never hard-fail) when supervised lanes
	// are not isolated from the daemon's PostgreSQL by a dedicated PG-less lane
	// OS user. Configuration-posture proxy only; no DSN/token value is read.
	laneSandboxBlock, laneSandboxWarnings := laneSandboxDoctorBlock()
	warnings = append(warnings, laneSandboxWarnings...)

	// RFC 0107: surface the configured principals and each one's capability/repo
	// scope so the operator can see who can do what, on which repositories, on
	// this self-hosted daemon. Daemon-global (independent of repository_id);
	// never reads or returns token material.
	principalsBlock := principalsDoctorBlock(ctx, runner)

	// RFC 0110: report the daemon->PostgreSQL write-boundary posture and the
	// bounded-discard reconnect signal. Posture is "none" in release N (no phase
	// has closed a surface); never reads or returns any secret.
	pgWriteBoundaryBlock, pgWriteBoundaryWarnings := pgWriteBoundaryDoctorBlock()
	warnings = append(warnings, pgWriteBoundaryWarnings...)

	return map[string]any{
		"ok":                  len(problems) == 0,
		"schema_version":      schemaVersion,
		"stale_leases":        staleLeases,
		"waiting_human":       waitingHuman,
		"needs_operator":      len(needsOperatorRuns),
		"needs_operator_runs": needsOperatorRuns,
		"supervisors":         supervisorLiveness,
		"problems":            problems,
		"warnings":            warnings,
		"codex":               codexBlock,
		"lane_sandbox":        laneSandboxBlock,
		"principals":          principalsBlock,
		"pg_write_boundary":   pgWriteBoundaryBlock,
		"blob":                blobDoctorBlock(ctx, runner, repositoryID),
	}, nil
}
