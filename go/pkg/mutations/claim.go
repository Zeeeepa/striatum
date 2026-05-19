package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func HandleClaimNext(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	if sessionID == "" {
		return nil, rpc.NewError("schema_invalid", "work.claim_next requires session_id", nil)
	}
	leaseSeconds := intParam(envelope, "lease_seconds", 3600)
	if leaseSeconds <= 0 {
		leaseSeconds = 3600
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		session, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, true)
		if err != nil {
			return nil, err
		}
		runID := fmt.Sprint(session["run_id"])
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		if _, err := expireLeases(ctx, tx, repositoryID, runID); err != nil {
			return nil, err
		}
		state := fmt.Sprint(run["state"])
		if state == "needs_branch_confirmation" || state == "ready" {
			return nil, rpc.NewError("branch_confirmation_required", "branch confirmation and run start are required before claims", nil)
		}
		if state != "running" {
			return map[string]any{"status": "no_work"}, nil
		}
		if nullable(run["paused_at"]) != nil {
			return map[string]any{"status": "no_work", "paused": true}, nil
		}
		rows, err := queryRows(ctx, tx, `
			SELECT qm.*
			  FROM striatumd.queue_messages qm
			  JOIN striatumd.jobs j
			    ON j.repository_id = qm.repository_id
			   AND j.job_id = qm.job_id
			 WHERE qm.repository_id = $1
			   AND qm.kind = 'work'
			   AND qm.state = 'pending'
			   AND qm.target_role_id = $2
			   AND (qm.target_lane_id IS NULL OR qm.target_lane_id = $3)
			   AND (
			     j.fresh_session_required = false
			     OR NOT EXISTS (
			       SELECT 1 FROM striatumd.work_packets wp
			        WHERE wp.repository_id = qm.repository_id
			          AND wp.run_id = qm.run_id
			          AND wp.session_id = $4
			     )
			   )
			   AND qm.run_id = $5
			 ORDER BY qm.priority DESC, qm.created_at ASC
			 LIMIT 1
			 FOR UPDATE OF qm SKIP LOCKED`,
			repositoryID,
			session["role_id"],
			session["lane_id"],
			sessionID,
			runID,
		)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return map[string]any{"status": "no_work"}, nil
		}
		chosen := rows[0]
		jobID := fmt.Sprint(chosen["job_id"])
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		now := nowString()
		leaseID, err := newID("lease")
		if err != nil {
			return nil, err
		}
		packetID, err := newID("wp")
		if err != nil {
			return nil, err
		}
		expiresAt := expiresAfter(leaseSeconds)
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.leases (
			  repository_id, lease_id, run_id, resource_type, resource_id,
			  owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
			)
			VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7,$8)`,
			repositoryID, leaseID, runID, jobID, sessionID, now, expiresAt, now); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.queue_messages
			   SET state = 'claimed', claimed_at = $1, updated_at = $2,
			       current_lease_id = $3, claim_count = claim_count + 1
			 WHERE repository_id = $4 AND message_id = $5`,
			now, now, leaseID, repositoryID, chosen["message_id"]); err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = 'claimed', current_lease_id = $1, started_at = $2
			 WHERE repository_id = $3 AND job_id = $4`,
			leaseID, now, repositoryID, jobID); err != nil {
			return nil, err
		}
		packet, err := buildPacket(ctx, tx, repositoryID, run, session, job, fmt.Sprint(chosen["message_id"]), leaseID, expiresAt, packetID)
		if err != nil {
			return nil, err
		}
		packetJSON, err := json.Marshal(packet)
		if err != nil {
			return nil, err
		}
		packetSum := sha256.Sum256(packetJSON)
		packetJSONArg, err := db.JSONBArg(tx, packet)
		if err != nil {
			return nil, err
		}
		if err := tx.Exec(ctx, `
			INSERT INTO striatumd.work_packets (
			  repository_id, packet_id, run_id, job_id, message_id, lease_id,
			  session_id, packet_json, packet_sha256, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)`,
			repositoryID,
			packetID,
			runID,
			jobID,
			chosen["message_id"],
			leaseID,
			sessionID,
			packetJSONArg,
			hex.EncodeToString(packetSum[:]),
			now,
		); err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "queue.claimed", sessionID, jobID, chosen["message_id"], nil, leaseID, nil); err != nil {
			return nil, err
		}
		return claimNextResult(sessionID, packetID, packet), nil
	})
}

func claimNextResult(sessionID, packetID string, packet map[string]any) map[string]any {
	return map[string]any{
		"status":    "claimed",
		"packet_id": packetID,
		"packet":    packet,
		"next_steps": map[string]any{
			"supervise_send": "striatum supervise send --session-id " + sessionID + " --packet-id " + packetID,
		},
	}
}

func buildPacket(
	ctx context.Context,
	runner any,
	repositoryID string,
	run map[string]any,
	session map[string]any,
	job map[string]any,
	messageID string,
	leaseID string,
	leaseExpiresAt string,
	packetID string,
) (map[string]any, error) {
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(run["workflow_snapshot_id"]), false)
	if err != nil {
		return nil, err
	}
	workflow := asMap(snapshot["workflow_json"])
	roles := asMap(workflow["roles"])
	roleDef := asMap(roles[fmt.Sprint(job["role_id"])])
	writeScope := asMap(job["write_scope_json"])
	expectedArtifacts := asList(job["expected_artifacts_json"])
	laneSelector := asMap(job["lane_selector_json"])
	laneID, _ := laneSelector["lane_id"].(string)
	if laneID == "" {
		laneID = fmt.Sprint(session["lane_id"])
	}
	lanes := asMap(workflow["lanes"])
	laneConfig := asMap(lanes[laneID])
	worktreeIsolation := laneWorktreeIsolation(workflow, laneID)
	worktreeRequired := worktreeIsolation == "per_job" && isRepoWrite(job)
	attestation := sessionLaneAttestation(ctx, runner, repositoryID, fmt.Sprint(session["session_id"]))
	attested, _ := attestation["attested"].(bool)
	operatorLabel, _ := nullable(session["operator_label"]).(string)
	author := artifactAuthorIdentity(
		workflow,
		fmt.Sprint(job["role_id"]),
		laneID,
		fmt.Sprint(job["workflow_job_id"]),
		intValue(session["ordinal"]),
		attested,
		operatorLabel,
	)
	authorLine, _ := author["line"].(string)
	if authorLine == "" {
		return nil, rpc.NewError("invalid_transition", "session author line could not be derived", nil)
	}
	requirements := asMap(job["capability_requirements_json"])
	packet := map[string]any{
		"packet_version": "striatum.work-packet.v1",
		"packet_id":      packetID,
		"run": map[string]any{
			"run_id":      run["run_id"],
			"workflow_id": workflow["workflow_id"],
			"repo_root":   run["repo_root"],
			"branch": map[string]any{
				"name":      run["branch_name"],
				"confirmed": nullable(run["branch_confirmed_at"]) != nil,
			},
		},
		"session": map[string]any{
			"session_id":   session["session_id"],
			"slug":         session["slug"],
			"role_id":      session["role_id"],
			"lane_id":      session["lane_id"],
			"capabilities": asList(session["capabilities_json"]),
		},
		"lane_attestation": attestation,
		"lease": map[string]any{
			"lease_id":                leaseID,
			"message_id":              messageID,
			"expires_at":              leaseExpiresAt,
			"heartbeat_after_seconds": 300,
		},
		"job": map[string]any{
			"job_id":                 job["job_id"],
			"workflow_job_id":        job["workflow_job_id"],
			"attempt":                job["attempt"],
			"type":                   job["job_type"],
			"title":                  job["title"],
			"author":                 author,
			"objective":              requirements["objective"],
			"fresh_session_required": boolValue(job["fresh_session_required"]),
		},
		"role": map[string]any{
			"role_id":         job["role_id"],
			"definition_path": roleDef["definition_path"],
			"inline_summary":  roleDef["summary"],
		},
		"context": map[string]any{
			"docs":         asList(workflow["context_docs"]),
			"content_mode": "references",
		},
		"task_prompt":         asMap(requirements["task_prompt"]),
		"inputs":              asList(requirements["inputs"]),
		"write_scope":         writeScope,
		"adapter_constraints": buildAdapterConstraints(laneConfig),
		"expected_artifacts":  expectedArtifactsWithAuthor(expectedArtifacts, authorLine),
		"worktree_isolation":  worktreeIsolation,
		"worktree_required":   worktreeRequired,
		"commands":            buildPacketCommands(fmt.Sprint(session["session_id"]), fmt.Sprint(job["job_id"]), messageID, leaseID, worktreeRequired),
		"artifact_policy": map[string]any{
			"publish_transcripts":    false,
			"curated_artifacts_only": true,
		},
	}
	if policy := buildReviewPolicy(workflow, fmt.Sprint(job["workflow_job_id"])); policy != nil {
		packet["review_policy"] = policy
	}
	if profile := harnessProfileView(workflow, laneID); profile != nil {
		packet["harness_profile"] = profile
	}
	return packet, nil
}

func laneWorktreeIsolation(workflow map[string]any, laneID string) string {
	if laneID == "" {
		return "off"
	}
	lane := asMap(asMap(workflow["lanes"])[laneID])
	mode, _ := lane["worktree_isolation"].(string)
	if mode == "per_job" {
		return "per_job"
	}
	return "off"
}

func expectedArtifactsWithAuthor(expected []any, authorLine string) []any {
	result := []any{}
	for _, item := range expected {
		artifact := asMap(item)
		if len(artifact) == 0 {
			result = append(result, item)
			continue
		}
		copy := map[string]any{}
		for key, value := range artifact {
			copy[key] = value
		}
		copy["author_line"] = authorLine
		result = append(result, copy)
	}
	return result
}

func buildPacketCommands(sessionID, jobID, messageID, leaseID string, worktreeRequired bool) map[string]any {
	commands := map[string]any{
		"ack":              fmt.Sprintf("striatum ack --session-id %s --message-id %s --lease-id %s", sessionID, messageID, leaseID),
		"heartbeat":        fmt.Sprintf("striatum heartbeat --session-id %s --lease-id %s", sessionID, leaseID),
		"publish_artifact": fmt.Sprintf("striatum publish-artifact --session-id %s --job-id %s --lease-id %s", sessionID, jobID, leaseID),
		"block":            fmt.Sprintf("striatum block --session-id %s --job-id %s --lease-id %s", sessionID, jobID, leaseID),
		"verdict":          fmt.Sprintf("striatum verdict --session-id %s --job-id %s --lease-id %s", sessionID, jobID, leaseID),
		"complete":         fmt.Sprintf("striatum complete --session-id %s --job-id %s --lease-id %s", sessionID, jobID, leaseID),
	}
	if worktreeRequired {
		commands["worktree_create"] = fmt.Sprintf("striatum worktree create --session-id %s --job-id %s --lease-id %s", sessionID, jobID, leaseID)
	}
	return commands
}

func buildAdapterConstraints(laneConfig map[string]any) map[string]any {
	adapter := laneConfig["adapter"]
	constraints := asMap(laneConfig["constraints"])
	required := asMap(laneConfig["required_enforcement"])
	enforcement := []any{}
	for key, value := range constraints {
		requested, ok := value.(string)
		if !ok {
			continue
		}
		actual := adapterConstraintEnforcement(adapter, key, requested)
		requiredText, _ := required[key].(string)
		enforcement = append(enforcement, map[string]any{
			"constraint":           key,
			"requested":            requested,
			"required_enforcement": nullable(requiredText),
			"enforcement":          actual,
			"satisfied":            requiredText == "" || adapterEnforcementSatisfies(actual, requiredText),
		})
	}
	satisfied := true
	for _, item := range enforcement {
		if asMap(item)["satisfied"] != true {
			satisfied = false
			break
		}
	}
	return map[string]any{
		"requested":            constraints,
		"required_enforcement": required,
		"enforcement":          enforcement,
		"satisfied":            satisfied,
	}
}

func adapterConstraintEnforcement(adapter any, constraint, requested string) string {
	if adapter == "process" {
		if constraint == "transcripts" && requested == "off" {
			return "enforced"
		}
		if constraint == "network" && requested == "forbidden" {
			return "advisory_strict"
		}
		if constraint == "repo_scope" && requested == "local_only" {
			return "advisory_strict"
		}
		return "advisory"
	}
	return "unsupported"
}

func adapterEnforcementSatisfies(actual, required string) bool {
	rank := map[string]int{"unsupported": 0, "advisory": 1, "advisory_strict": 2, "enforced": 3}
	return rank[actual] >= rank[required]
}

func harnessProfileView(workflow map[string]any, laneID string) map[string]any {
	if laneID == "" {
		return nil
	}
	lane := asMap(asMap(workflow["lanes"])[laneID])
	profileID, _ := lane["harness_profile_id"].(string)
	if profileID == "" {
		return nil
	}
	body := asMap(asMap(workflow["harness_profiles"])[profileID])
	if len(body) == 0 {
		return nil
	}
	view := map[string]any{"profile_id": profileID}
	for key, value := range body {
		view[key] = value
	}
	return view
}

var reviewAccessInstructions = map[string]string{
	"document_only":      "Read only the target documents listed in inputs. Do not consult other artifacts, ledgers, reports, or repository contents beyond inputs.",
	"artifact_augmented": "You may read the target documents AND the supporting artifacts/reports/ledgers listed in inputs. Do not inspect other repository contents.",
	"repo_level":         "You may inspect the repository within the job's declared write_scope.allowed_paths/forbidden_paths. Stay within that scope.",
}

var reviewContextInstructions = map[string]string{
	"fresh":       " This is a fresh-context review. Do not rely on prior thread state from earlier rounds.",
	"cross_round": " You may retain prior context to verify whether previously raised issues were resolved.",
}

var reviewPostureInstructions = map[string]string{
	"neutral":             "",
	"devils_advocate":     " This is a devil's-advocate review. Argue against the artifact's claims; verdict acceptance means the claims survived your strongest counterarguments.",
	"security":            " This is a security-focused review. Read the artifact looking for security weaknesses; verdict acceptance means you actively looked and found nothing actionable.",
	"threat_model":        " This is a threat-modeling review. Enumerate the trust boundaries and attack surfaces the artifact introduces; verdict acceptance means each is acknowledged or mitigated.",
	"latency_performance": " This is a latency / performance review. Evaluate the artifact's runtime and resource cost; verdict acceptance means no acceptance-blocking regression was found.",
	"ergonomics_dx":       " This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent.",
	"accessibility":       " This is an accessibility review. Evaluate the artifact against accessibility expectations; verdict acceptance means the affordances meet the declared accessibility bar.",
	"compliance_license":  " This is a compliance / license review. Evaluate the artifact for license, attribution, or compliance issues; verdict acceptance means none are unresolved.",
	"supply_chain":        " This is a supply-chain review. Evaluate the artifact's external dependencies and their provenance; verdict acceptance means each is justified and pinned.",
}

func buildReviewPolicy(workflow map[string]any, workflowJobID string) map[string]any {
	for _, item := range asList(workflow["jobs"]) {
		job := asMap(item)
		if job["id"] != workflowJobID || job["type"] != "review" {
			continue
		}
		_, hasAccess := job["reviewer_access_scope"]
		_, hasContext := job["reviewer_context_policy"]
		_, hasPosture := job["review_posture"]
		if !hasAccess && !hasContext && !hasPosture {
			return nil
		}
		access, _ := job["reviewer_access_scope"].(string)
		if access == "" {
			access = "document_only"
		}
		contextPolicy, _ := job["reviewer_context_policy"].(string)
		if contextPolicy == "" {
			contextPolicy = "cross_round"
		}
		posture, _ := job["review_posture"].(string)
		if posture == "" {
			posture = "neutral"
		}
		accessText, ok := reviewAccessInstructions[access]
		if !ok {
			return nil
		}
		contextText, ok := reviewContextInstructions[contextPolicy]
		if !ok {
			return nil
		}
		block := map[string]any{
			"access_scope":   access,
			"context_policy": contextPolicy,
			"instruction":    accessText + contextText + reviewPostureInstructions[posture],
		}
		if hasPosture {
			block["posture"] = posture
		}
		return block
	}
	return nil
}

func artifactAuthorIdentity(workflow map[string]any, roleID, laneID, workflowJobID string, ordinal int, attested bool, operatorLabel string) map[string]any {
	displayModel := laneDisplayModel(workflow, laneID)
	var line any
	if ordinal > 0 {
		if attested {
			model := displayModel
			if model == "" {
				model = "unknown-model"
			}
			line = fmt.Sprintf("author: %s-%s-%03d", authorPart(roleID), authorPart(model), ordinal)
		} else {
			line = operatorAuthorLine(operatorLabel)
		}
	}
	return map[string]any{
		"role_id":         roleID,
		"lane_id":         nullable(laneID),
		"display_model":   nullable(displayModel),
		"workflow_job_id": workflowJobID,
		"ordinal":         ordinal,
		"line":            line,
	}
}

func laneDisplayModel(workflow map[string]any, laneID string) string {
	if laneID == "" {
		return ""
	}
	model, _ := asMap(asMap(workflow["lanes"])[laneID])["display_model"].(string)
	return model
}

var authorPartPattern = regexp.MustCompile(`[^a-z0-9.]+`)

func authorPart(value string) string {
	normalized := strings.Trim(authorPartPattern.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func operatorAuthorLine(operatorLabel string) string {
	if operatorLabel == "" || operatorLabel == "<nil>" {
		return "author: operator"
	}
	return fmt.Sprintf("author: operator [self-declared: %s]", operatorLabel)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	}
	return 0
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case string:
		return typed == "true" || typed == "1"
	}
	return false
}
