package mutations

import (
	"context"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
)

// freshReviewLineageEvent is the audit event recorded when a claim is refused
// because the claimant's supervised process is not a genuinely fresh reviewer.
const freshReviewLineageEvent = "work.claim_ineligible_fresh_review_lineage"

// freshReviewProcessLineageRefusal enforces fresh-review PROCESS independence at
// work.claim (#222). The existing fresh-session gate only guarantees a new
// session id; a long-lived lane process can register a fresh session and review
// its own upstream work, recording a contaminated verdict. For a fresh-context
// review job this gate additionally requires that the claimant's supervised
// process (process_supervisor_pointers.pid + pid_start_time) did not do durable
// work upstream of the review in this run.
//
// It returns a non-nil structured no_work refusal (ineligible_reason
// fresh_review_process_lineage) when the claim must be refused, leaving the job
// queued for a real fresh reviewer; nil means the claim may proceed. It is run
// after the fresh-session gate and before any lease/packet is created.
func freshReviewProcessLineageRefusal(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID string, job map[string]any) (map[string]any, error) {
	if !boolValue(job["fresh_session_required"]) || !isVerdictCapableJobType(fmt.Sprint(job["job_type"])) {
		return nil, nil
	}
	jobID := fmt.Sprint(job["job_id"])

	pid, startToken, found, err := sessionSupervisedProcessIdentity(ctx, tx, repositoryID, sessionID)
	if err != nil {
		return nil, err
	}
	if !found || pid <= 0 || strings.TrimSpace(startToken) == "" {
		// A supervised fresh-review lane whose process identity cannot be verified
		// must not claim either: the daemon cannot prove reviewer independence.
		return freshReviewLineageRefusal(ctx, tx, repositoryID, runID, sessionID, jobID, map[string]any{
			"identity_status": "missing_or_unverified",
			"hint":            "this fresh-review job requires a verifiable supervised process identity (pid + pid_start_time) to prove reviewer independence; register/start a fresh supervised session and retry.",
		})
	}

	upstreamJobIDs, err := upstreamReviewJobIDs(ctx, tx, repositoryID, runID, jobID)
	if err != nil {
		return nil, err
	}
	if len(upstreamJobIDs) == 0 {
		return nil, nil
	}
	contaminating, err := upstreamWorkByProcess(ctx, tx, repositoryID, upstreamJobIDs, pid, startToken)
	if err != nil {
		return nil, err
	}
	if contaminating == nil {
		return nil, nil
	}
	return freshReviewLineageRefusal(ctx, tx, repositoryID, runID, sessionID, jobID, map[string]any{
		"contaminating_session_id": contaminating["session_id"],
		"contaminating_job_id":     contaminating["job_id"],
		"process_pid":              pid,
		"process_pid_start_time":   startToken,
		"hint":                     "the supervised process claiming this fresh review already did durable work upstream of it in this run; a genuinely fresh reviewer process must claim it. The job remains queued.",
	})
}

// sessionSupervisedProcessIdentity returns the claimant session's current
// supervised process identity (pid + pid_start_time). found is false when the
// session has no supervisor pointer at all.
func sessionSupervisedProcessIdentity(ctx context.Context, tx db.TxRunner, repositoryID, sessionID string) (pid int, startToken string, found bool, err error) {
	rows, err := queryRows(ctx, tx, `
		SELECT pid, pid_start_time
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND session_id = $2
		   AND state IN ('starting','attached','detached')
		 ORDER BY updated_at DESC
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return 0, "", false, err
	}
	if len(rows) == 0 {
		return 0, "", false, nil
	}
	row := rows[0]
	startToken = ""
	if value := row["pid_start_time"]; value != nil {
		startToken = strings.TrimSpace(fmt.Sprint(value))
	}
	return intValue(row["pid"]), startToken, true, nil
}

// upstreamReviewJobIDs returns every job_id the review depends on, expanded to
// all attempts of those upstream workflow jobs so a process that did an earlier
// attempt is still recognized as upstream contamination.
func upstreamReviewJobIDs(ctx context.Context, tx db.TxRunner, repositoryID, runID, reviewJobID string) ([]string, error) {
	rows, err := queryRows(ctx, tx, `
		SELECT DISTINCT j2.job_id
		  FROM striatumd.job_dependencies dep
		  JOIN striatumd.jobs j1
		    ON j1.repository_id = dep.repository_id AND j1.job_id = dep.depends_on_job_id
		  JOIN striatumd.jobs j2
		    ON j2.repository_id = dep.repository_id
		   AND j2.run_id = j1.run_id
		   AND j2.workflow_job_id = j1.workflow_job_id
		 WHERE dep.repository_id = $1 AND dep.job_id = $2`, repositoryID, reviewJobID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(fmt.Sprint(row["job_id"])); id != "" && id != reviewJobID {
			out = append(out, id)
		}
	}
	return out, nil
}

// upstreamWorkByProcess returns the first upstream (session_id, job_id) whose
// work packet was claimed by the given supervised process identity, or nil if
// that process did no durable upstream work. A claimed work packet — not mere
// session registration — is the contamination signal.
func upstreamWorkByProcess(ctx context.Context, tx db.TxRunner, repositoryID string, upstreamJobIDs []string, pid int, startToken string) (map[string]any, error) {
	rows, err := queryRows(ctx, tx, `
		SELECT wp.session_id, wp.job_id
		  FROM striatumd.work_packets wp
		  JOIN striatumd.process_supervisor_pointers p
		    ON p.repository_id = wp.repository_id AND p.session_id = wp.session_id
		 WHERE wp.repository_id = $1
		   AND wp.job_id = ANY($2)
		   AND p.pid = $3
		   AND p.pid_start_time = $4
		 LIMIT 1`, repositoryID, upstreamJobIDs, pid, startToken)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// freshReviewLineageRefusal records a deduped audit event for the ineligible
// claim attempt and returns the structured no_work refusal.
func freshReviewLineageRefusal(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID, jobID string, detail map[string]any) (map[string]any, error) {
	if err := recordFreshReviewLineageAudit(ctx, tx, repositoryID, runID, sessionID, jobID, detail); err != nil {
		return nil, err
	}
	res := map[string]any{
		"status":            "no_work",
		"ineligible_reason": "fresh_review_process_lineage",
		"job_id":            jobID,
	}
	for key, value := range detail {
		res[key] = value
	}
	return res, nil
}

// recordFreshReviewLineageAudit appends the ineligible-claim audit event once
// per (session, job): a polling lane retries work.claim, so the event is deduped
// to avoid flooding the event log.
func recordFreshReviewLineageAudit(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID, jobID string, detail map[string]any) error {
	existing, err := queryRows(ctx, tx, `
		SELECT 1
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = $3
		   AND actor_session_id = $4 AND job_id = $5
		 LIMIT 1`, repositoryID, runID, freshReviewLineageEvent, sessionID, jobID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	payload := map[string]any{"ineligible_reason": "fresh_review_process_lineage"}
	for key, value := range detail {
		payload[key] = value
	}
	_, err = appendEvent(ctx, tx, repositoryID, runID, freshReviewLineageEvent, sessionID, jobID, nil, nil, nil, payload)
	return err
}
