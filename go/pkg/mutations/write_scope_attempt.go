package mutations

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

const writeScopeDriftMessage = "write_scope violation: this job attempt's effective write_scope is frozen at claim time. Use the path in the active work packet, clear or move out-of-scope changes, or route through audited recovery (`striatum recovery resume` for write-scope blockers, or a replacement/fresh attempt for legitimate scope changes); do not mutate historical scope for this attempt."

func frozenAttemptWriteScope(ctx context.Context, runner any, repositoryID string, job map[string]any, leaseID string) (scope map[string]any, baseline map[string]any, found bool) {
	jobID := fmt.Sprint(job["job_id"])
	attempt := jobAttemptValue(job["attempt"])
	if leaseID != "" {
		if packet, ok := workPacketForLease(ctx, runner, repositoryID, jobID, leaseID); ok && packetAttempt(packet) == attempt {
			return packetWriteScope(packet), asMap(packet["write_scope_baseline"]), true
		}
	}
	rows, err := queryRows(ctx, runner, `
		SELECT packet_json
		  FROM striatumd.work_packets
		 WHERE repository_id = $1 AND job_id = $2
		 ORDER BY created_at, packet_id`, repositoryID, jobID)
	if err != nil {
		return nil, nil, false
	}
	for _, row := range rows {
		packet := asMap(row["packet_json"])
		if packetAttempt(packet) != attempt {
			continue
		}
		return packetWriteScope(packet), asMap(packet["write_scope_baseline"]), true
	}
	return nil, nil, false
}

func applyFrozenAttemptWriteScope(ctx context.Context, runner any, repositoryID string, job map[string]any, leaseID string) {
	scope, baseline, found := frozenAttemptWriteScope(ctx, runner, repositoryID, job, leaseID)
	if !found {
		return
	}
	if len(scope) > 0 {
		job["write_scope_json"] = scope
	}
	if len(baseline) > 0 {
		job["write_scope_baseline"] = baseline
	}
}

func workPacketForLease(ctx context.Context, runner any, repositoryID, jobID, leaseID string) (map[string]any, bool) {
	row, err := oneRow(ctx, runner, `
		SELECT packet_json
		  FROM striatumd.work_packets
		 WHERE repository_id = $1 AND job_id = $2 AND lease_id = $3
		 ORDER BY created_at DESC, packet_id DESC
		 LIMIT 1`, repositoryID, jobID, leaseID)
	if err != nil {
		return nil, false
	}
	return asMap(row["packet_json"]), true
}

func packetAttempt(packet map[string]any) int {
	return jobAttemptValue(asMap(packet["job"])["attempt"])
}

func packetWriteScope(packet map[string]any) map[string]any {
	return asMap(packet["write_scope"])
}

func writeScopePathError(job map[string]any, pathText string, allowed, forbidden []string) error {
	return rpc.NewError("write_scope_drift", writeScopeDriftMessage, map[string]any{
		"job_id":          job["job_id"],
		"workflow_job_id": job["workflow_job_id"],
		"path":            pathText,
		"allowed_paths":   allowed,
		"forbidden_paths": forbidden,
		"recovery":        "Use `striatum recovery resume` for write-scope blockers after remediation, or start a fresh/replacement attempt for legitimate scope changes.",
	})
}
