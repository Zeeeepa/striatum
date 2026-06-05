package workflowauthoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

var modelTokenPattern = regexp.MustCompile(`[^a-z0-9]+`)

func WorkflowFingerprint(workflow map[string]any) (string, error) {
	normalized, err := json.Marshal(workflow)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:]), nil
}

func Lint(workflow map[string]any) (map[string]any, error) {
	if err := Validate(workflow); err != nil {
		return map[string]any{
			"workflow_id":   stringValue(workflow["workflow_id"]),
			"valid":         false,
			"errors":        []map[string]any{workflowErrorMap(err)},
			"warnings":      []map[string]any{},
			"warning_count": 0,
			"coverage":      invalidLintCoverage(),
		}, nil
	}
	jobMap := WorkflowJobMap(workflow)
	findings := []map[string]any{}
	lintSameModelReviewPairs(workflow, jobMap, &findings)
	lintReviewFreshness(jobMap, &findings)
	lintWriteScopeRisk(workflow, jobMap, &findings)
	lintParallelSharedResources(jobMap, &findings)
	lintMissingEscalationPath(workflow, jobMap, &findings)
	lintAgyOneShotPipeLane(workflow, &findings)
	lintDeprecatedClaudePrintLane(workflow, &findings)
	lintExperimentalShape(workflow, &findings)
	lintDegradedSeatLane(workflow, &findings)
	lintInterrogationTargets(workflow, jobMap, &findings)
	for index := range findings {
		findings[index]["fingerprint"] = FindingFingerprint(findings[index])
	}
	return map[string]any{
		"workflow_id":   stringValue(workflow["workflow_id"]),
		"valid":         true,
		"errors":        []map[string]any{},
		"warnings":      findings,
		"warning_count": len(findings),
		"coverage":      lintCoverage(workflow, jobMap, findings),
	}, nil
}

func FindingFingerprint(finding map[string]any) string {
	normalized := map[string]any{}
	for key, value := range finding {
		if key == "fingerprint" {
			continue
		}
		normalized[key] = value
	}
	payload, _ := json.Marshal(map[string]any{
		"rule":    stringValue(normalized["rule"]),
		"finding": normalized,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func workflowErrorMap(err error) map[string]any {
	result := map[string]any{"message": err.Error()}
	if workflowErr, ok := err.(*Error); ok && workflowErr.FieldPath != "" {
		result["field_path"] = workflowErr.FieldPath
	}
	return result
}

func lintSameModelReviewPairs(workflow map[string]any, jobMap map[string]map[string]any, findings *[]map[string]any) {
	lanes, ok := workflow["lanes"].(map[string]any)
	if !ok {
		return
	}
	emitted := map[string]bool{}
	familyForJob := func(job map[string]any) string {
		laneID := stringValue(job["lane_id"])
		if laneID == "" {
			return ""
		}
		lane, ok := lanes[laneID].(map[string]any)
		if !ok {
			return ""
		}
		source := stringValue(lane["display_model"])
		if source == "" {
			source = laneID
		}
		return modelFamily(source)
	}
	edges, err := EdgeDependencyPairs(workflow)
	if err == nil {
		for _, edge := range edges {
			fromID := stringValue(edge["from"])
			toID := stringValue(edge["to"])
			upstream := jobMap[fromID]
			reviewJob := jobMap[toID]
			if defaultString(reviewJob["type"], "generic") != "review" || verdictJobTypes[defaultString(upstream["type"], "generic")] {
				continue
			}
			upstreamFamily := familyForJob(upstream)
			reviewFamily := familyForJob(reviewJob)
			if upstreamFamily == "" || reviewFamily == "" || upstreamFamily != reviewFamily {
				continue
			}
			key := "edge\x00" + fromID + "\x00" + toID
			if emitted[key] {
				continue
			}
			emitted[key] = true
			*findings = append(*findings, map[string]any{
				"rule":           "same_model_review_pair",
				"severity":       "warning",
				"message":        "review job '" + toID + "' and upstream job '" + fromID + "' use the same model family '" + reviewFamily + "'; use an independent review lane or record an explicit override rationale",
				"job_id":         toID,
				"related_job_id": fromID,
				"model_family":   reviewFamily,
			})
		}
		for _, edge := range edges {
			fromID := stringValue(edge["from"])
			toID := stringValue(edge["to"])
			upstream := jobMap[fromID]
			adjudicatorJob := jobMap[toID]
			if upstream == nil || adjudicatorJob == nil || !isCollaborationAdjudicatorJob(adjudicatorJob) {
				continue
			}
			upstreamFamily := familyForJob(upstream)
			adjudicatorFamily := familyForJob(adjudicatorJob)
			if upstreamFamily == "" || adjudicatorFamily == "" || upstreamFamily != adjudicatorFamily {
				continue
			}
			key := "adjudicator\x00" + fromID + "\x00" + toID
			if emitted[key] {
				continue
			}
			emitted[key] = true
			*findings = append(*findings, map[string]any{
				"rule":           "same_model_adjudicator_pair",
				"severity":       "warning",
				"message":        "adjudicator job '" + toID + "' and upstream job '" + fromID + "' use the same model family '" + adjudicatorFamily + "'; use an independent adjudicator lane or record an explicit override rationale",
				"job_id":         toID,
				"related_job_id": fromID,
				"model_family":   adjudicatorFamily,
			})
		}
	}
	for _, item := range anySlice(workflow["cycles"]) {
		cycle, ok := item.(map[string]any)
		if !ok || cycle["allow_same_model"] == true {
			continue
		}
		cycleFrom := stringValue(cycle["from"])
		cycleTo := stringValue(cycle["to"])
		cycleReview := jobMap[cycleFrom]
		implementer := jobMap[cycleTo]
		if cycleReview == nil || implementer == nil || !verdictJobTypes[defaultString(cycleReview["type"], "generic")] {
			continue
		}
		reviewFamily := familyForJob(cycleReview)
		implementerFamily := familyForJob(implementer)
		if reviewFamily == "" || implementerFamily == "" || reviewFamily != implementerFamily {
			continue
		}
		key := "cycle\x00" + cycleFrom + "\x00" + cycleTo
		if emitted[key] {
			continue
		}
		emitted[key] = true
		*findings = append(*findings, map[string]any{
			"rule":           "same_model_revision_cycle",
			"severity":       "warning",
			"message":        "revision cycle '" + cycleFrom + "' -> '" + cycleTo + "' returns review to the same model family '" + reviewFamily + "'; set cycle.allow_same_model=true only with an override rationale",
			"job_id":         cycleFrom,
			"related_job_id": cycleTo,
			"model_family":   reviewFamily,
		})
	}
}

func isCollaborationAdjudicatorJob(job map[string]any) bool {
	if stringValue(job["role_id"]) == "adjudicator" {
		return true
	}
	for _, item := range anySlice(job["expected_artifacts"]) {
		artifact, ok := item.(map[string]any)
		if ok && stringValue(artifact["kind"]) == "collaboration_ledger" {
			return true
		}
	}
	return false
}

func lintReviewFreshness(jobMap map[string]map[string]any, findings *[]map[string]any) {
	for _, jobID := range sortedJobIDs(jobMap) {
		job := jobMap[jobID]
		if defaultString(job["type"], "generic") != "review" {
			continue
		}
		if effectiveFreshSessionRequired(job) || job["reviewer_context_policy"] == "fresh" {
			continue
		}
		*findings = append(*findings, map[string]any{
			"rule":     "review_without_fresh_context",
			"severity": "warning",
			"message":  "review job '" + jobID + "' does not require a fresh session; fresh review context reduces reviewer contamination",
			"job_id":   jobID,
		})
	}
}

func lintWriteScopeRisk(workflow map[string]any, jobMap map[string]map[string]any, findings *[]map[string]any) {
	laneMap, _ := workflow["lanes"].(map[string]any)
	for _, jobID := range sortedJobIDs(jobMap) {
		job := jobMap[jobID]
		scope, ok := job["write_scope"].(map[string]any)
		if !ok {
			continue
		}
		repoWrite := scope["repo_write"] == true || stringValue(scope["mode"]) == "repo_write"
		if !repoWrite {
			continue
		}
		allowed := stringsFromSlice(scope["allowed_paths"])
		broad := len(allowed) == 0
		for _, item := range allowed {
			if item == "" || item == "." || item == "./" || item == "/" {
				broad = true
			}
		}
		if broad {
			*findings = append(*findings, map[string]any{
				"rule":     "broad_write_scope",
				"severity": "warning",
				"message":  "repo-write job '" + jobID + "' has broad or empty allowed_paths; narrow write scope before running untrusted changes",
				"job_id":   jobID,
			})
		}
		laneID := stringValue(job["lane_id"])
		lane, _ := laneMap[laneID].(map[string]any)
		if lane == nil || lane["worktree_isolation"] != "per_job" {
			*findings = append(*findings, map[string]any{
				"rule":     "repo_write_without_worktree_isolation",
				"severity": "warning",
				"message":  "repo-write job '" + jobID + "' is not on a per-job worktree lane; parallel or revision work can collide in the main worktree",
				"job_id":   jobID,
			})
		}
	}
}

type sharedResourceClaim struct {
	jobID     string
	mode      string
	namespace string
}

func lintParallelSharedResources(jobMap map[string]map[string]any, findings *[]map[string]any) {
	groups := map[string][]map[string]any{}
	for _, jobID := range sortedJobIDs(jobMap) {
		job := jobMap[jobID]
		group := stringValue(job["parallel_group"])
		if group == "" {
			continue
		}
		groups[group] = append(groups[group], job)
	}
	for _, group := range sortedKeys(groups) {
		claimsByResource := map[string][]sharedResourceClaim{}
		for _, job := range groups[group] {
			jobID := stringValue(job["id"])
			for _, resource := range plannedSharedResources(job) {
				id := stringValue(resource["id"])
				if id == "" {
					continue
				}
				claimsByResource[id] = append(claimsByResource[id], sharedResourceClaim{
					jobID:     jobID,
					mode:      defaultString(resource["mode"], "exclusive"),
					namespace: stringValue(resource["namespace"]),
				})
			}
		}
		for _, resourceID := range sortedKeysFromClaims(claimsByResource) {
			claims := claimsByResource[resourceID]
			if len(claims) < 2 {
				continue
			}
			if exclusiveSharedResourceJobs(claims, group, resourceID, findings) {
				continue
			}
			namespaceSharedResourceJobs(claims, group, resourceID, findings)
		}
	}
}

func exclusiveSharedResourceJobs(claims []sharedResourceClaim, group string, resourceID string, findings *[]map[string]any) bool {
	hasExclusive := false
	jobIDs := []string{}
	for _, claim := range claims {
		if claim.mode == "exclusive" {
			hasExclusive = true
		}
		jobIDs = append(jobIDs, claim.jobID)
	}
	if !hasExclusive {
		return false
	}
	sort.Strings(jobIDs)
	*findings = append(*findings, map[string]any{
		"rule":           "parallel_shared_resource_contention",
		"severity":       "warning",
		"message":        "parallel group '" + group + "' has jobs sharing exclusive resource '" + resourceID + "'; serialize those jobs or declare distinct per-lane namespaces",
		"parallel_group": group,
		"resource_id":    resourceID,
		"job_ids":        jobIDs,
	})
	return true
}

func namespaceSharedResourceJobs(claims []sharedResourceClaim, group string, resourceID string, findings *[]map[string]any) {
	byNamespace := map[string][]string{}
	for _, claim := range claims {
		if claim.mode != "per_lane_namespace" || claim.namespace == "" {
			continue
		}
		byNamespace[claim.namespace] = append(byNamespace[claim.namespace], claim.jobID)
	}
	for _, namespace := range sortedKeysFromStringSlices(byNamespace) {
		jobIDs := byNamespace[namespace]
		if len(jobIDs) < 2 {
			continue
		}
		sort.Strings(jobIDs)
		*findings = append(*findings, map[string]any{
			"rule":           "parallel_shared_resource_contention",
			"severity":       "warning",
			"message":        "parallel group '" + group + "' has jobs sharing resource '" + resourceID + "' namespace '" + namespace + "'; use distinct namespaces or serialize the jobs",
			"parallel_group": group,
			"resource_id":    resourceID,
			"namespace":      namespace,
			"job_ids":        jobIDs,
		})
	}
}

func lintMissingEscalationPath(workflow map[string]any, jobMap map[string]map[string]any, findings *[]map[string]any) {
	hasReview := false
	for _, job := range jobMap {
		if verdictJobTypes[defaultString(job["type"], "generic")] {
			hasReview = true
			break
		}
	}
	if !hasReview {
		return
	}
	if policy, ok := workflow["review_revision_policy"].(map[string]any); ok && policy["root_review_needs_revision"] == "human_checkpoint" {
		return
	}
	for _, item := range anySlice(workflow["cycles"]) {
		if cycle, ok := item.(map[string]any); ok && cycle["on_verdict"] == "needs_revision" {
			return
		}
	}
	*findings = append(*findings, map[string]any{
		"rule":     "missing_review_escalation_path",
		"severity": "warning",
		"message":  "workflow has review jobs but no needs_revision cycle or review_revision_policy.root_review_needs_revision=human_checkpoint",
	})
}

// lintAgyOneShotPipeLane warns when an `agy` (Antigravity) lane is configured
// as a one-shot pipe lane (`agy … --print`, typically with a stdin shim or
// supervision.stdin_delivery=one_shot_eof) without declaring
// adapter_capabilities.agent_loop=true. The one-shot pipe path gives agy no
// auto-MCP config and no auto-delivery, so it launches, reads nothing on
// stdin, runs `agy --print ""` (empty), and exits without claiming (#51,
// #63 F5). agy self-claims only as an agent-loop lane. claude/codex one-shot
// pipe lanes self-claim and are not flagged: the check requires the command
// to invoke the `agy` binary with `--print`.
func lintAgyOneShotPipeLane(workflow map[string]any, findings *[]map[string]any) {
	lanes, ok := workflow["lanes"].(map[string]any)
	if !ok {
		return
	}
	for _, laneID := range sortedLaneIDs(lanes) {
		lane, ok := lanes[laneID].(map[string]any)
		if !ok {
			continue
		}
		if laneDeclaresAgentLoop(lane) {
			continue
		}
		if !laneCommandIsAgyPrint(lane) {
			continue
		}
		*findings = append(*findings, map[string]any{
			"rule":     "agy_one_shot_pipe_lane",
			"severity": "warning",
			"message":  "lane '" + laneID + "' runs `agy --print` as a one-shot pipe lane without adapter_capabilities.agent_loop=true; agy does not self-claim on the one-shot path (no auto-MCP config, no auto-delivery). Configure agy as an agent-loop lane: command [\"agy\", \"--dangerously-skip-permissions\"] with \"adapter_capabilities\": {\"agent_loop\": true} (see #51, #63 F5)",
			"lane_id":  laneID,
		})
	}
}

// lintExperimentalShape warns (RFC 0106) when a workflow declares a generator
// shape that has no unattended-reliability gate (support_tier=experimental). It
// does NOT block — yolo may opt in knowingly — but the default run path surfaces
// the risk. A workflow with no declared `shape` (hand-authored) is not
// classified and produces no warning.
func lintExperimentalShape(workflow map[string]any, findings *[]map[string]any) {
	shape := stringValue(workflow["shape"])
	if shape == "" {
		return
	}
	if workflowtemplates.SupportTierForShape(shape) != workflowtemplates.SupportTierExperimental {
		return
	}
	*findings = append(*findings, map[string]any{
		"rule":     "experimental_shape",
		"severity": "warning",
		"message":  fmt.Sprintf("workflow shape %q is experimental: it has no unattended-reliability gate (RFC 0105), so a multi-lane / revision run may wedge without recovery — expect to supervise it, or choose a `supported` shape. See RFC 0106.", shape),
		"shape":    shape,
	})
}

// lintDegradedSeatLane warns (RFC 0109 P2) when a workflow declares a lane whose
// adapter SEAT is `degraded` or `unsupported` — a seat with a known, tracked
// defect that prevents it holding a reliable supervised multi-turn lane (today:
// `agy`, #95/#85/#76/#139). Without this, a declared 3-lane panel silently
// collapses to 2 when the agy seat trust-gates and multi-turn-crashes out (#139):
// the workflow SAYS three, the run DELIVERS two, and nothing fails. The warning
// makes that degradation a recorded, surfaced event instead.
//
// It does NOT block (yolo may opt in knowingly) and it does NOT warn on
// `experimental` seats (claude/codex: they hold a seat, just ungated by an
// installed-CLI fixture) — faithful to RFC 0109's acceptance, which surfaces
// `degraded`/`unsupported` seats specifically. The seat may only graduate to
// `supported` once its RFC 0109 P3 installed-CLI conformance fixture is green
// (#149), enforced by the adapterconformance graduation guard.
func lintDegradedSeatLane(workflow map[string]any, findings *[]map[string]any) {
	lanes, ok := workflow["lanes"].(map[string]any)
	if !ok {
		return
	}
	for _, laneID := range sortedLaneIDs(lanes) {
		lane, ok := lanes[laneID].(map[string]any)
		if !ok {
			continue
		}
		adapter := laneAdapter(lane)
		if adapter == "" {
			continue
		}
		tier := workflowtemplates.SeatTierForAdapter(adapter)
		if tier != workflowtemplates.SeatTierDegraded && tier != workflowtemplates.SeatTierUnsupported {
			continue
		}
		message := fmt.Sprintf("lane %q declares the %q adapter whose supervised seat is %s: %s. A workflow that declares this lane may silently deliver one fewer voice than it names (#139) — expect to supervise it, or use a seat that holds. See RFC 0109.",
			laneID, adapter, tier, workflowtemplates.SeatDegradationReason(adapter))
		*findings = append(*findings, map[string]any{
			"rule":      "degraded_seat_lane",
			"severity":  "warning",
			"message":   message,
			"lane_id":   laneID,
			"adapter":   adapter,
			"seat_tier": tier,
		})
	}
}

func lintInterrogationTargets(workflow map[string]any, jobMap map[string]map[string]any, findings *[]map[string]any) {
	lanes, _ := workflow["lanes"].(map[string]any)
	directDeps := map[string]bool{}
	for _, item := range anySlice(workflow["edges"]) {
		edge, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		if fromID != "" && toID != "" {
			directDeps[toID+"\x00"+fromID] = true
		}
	}
	for _, jobID := range sortedJobIDs(jobMap) {
		job := jobMap[jobID]
		targets := anySlice(job["interrogation_targets"])
		if len(targets) == 0 {
			continue
		}
		if len(targets) > 3 {
			*findings = append(*findings, map[string]any{
				"rule":         "interrogation_target_count_high",
				"severity":     "warning",
				"message":      fmt.Sprintf("job %q declares %d interrogation targets; more than three targets increases operator and lane complexity", jobID, len(targets)),
				"job_id":       jobID,
				"target_count": len(targets),
			})
		}
		laneID := stringValue(job["lane_id"])
		if !laneHasCapability(lanes, laneID, "interrogate") {
			*findings = append(*findings, map[string]any{
				"rule":     "interrogation_target_missing_interrogate_capability",
				"severity": "warning",
				"message":  fmt.Sprintf("job %q declares interrogation_targets but lane %q does not declare the interrogate capability", jobID, laneID),
				"job_id":   jobID,
				"lane_id":  laneID,
			})
		}
		for index, item := range targets {
			target, ok := item.(map[string]any)
			if !ok {
				continue
			}
			targetID := stringValue(target["workflow_job_id"])
			for key := range target {
				if key == "workflow_job_id" || key == "required" {
					continue
				}
				*findings = append(*findings, map[string]any{
					"rule":          "interrogation_target_unknown_field",
					"severity":      "warning",
					"message":       fmt.Sprintf("job %q interrogation_targets[%d] has unknown field %q; V1 ignores it", jobID, index, key),
					"job_id":        jobID,
					"target_job_id": targetID,
					"field":         key,
				})
			}
			if directDeps[jobID+"\x00"+targetID] {
				*findings = append(*findings, map[string]any{
					"rule":           "interrogation_target_redundant_direct_dependency",
					"severity":       "warning",
					"message":        fmt.Sprintf("job %q declares interrogation target %q that is already a direct dependency", jobID, targetID),
					"job_id":         jobID,
					"target_job_id":  targetID,
					"related_job_id": targetID,
				})
			}
		}
	}
}

func laneHasCapability(lanes map[string]any, laneID string, capability string) bool {
	lane, ok := lanes[laneID].(map[string]any)
	if !ok {
		return false
	}
	for _, item := range anySlice(lane["capabilities"]) {
		if stringValue(item) == capability {
			return true
		}
	}
	return false
}

// laneAdapter resolves a lane's bare adapter seat name from its command argv0
// (basename, drop a .exe suffix), matching agentloop.LaneAdapterName and
// workflowtemplates.normalizeAdapterName. A lane with no command yields "".
// Shell-shim one-shot lanes (argv0 = sh/bash) resolve to the interpreter, not the
// inner CLI — those one-shot agy footguns are already covered by
// lintAgyOneShotPipeLane; this rule targets the declared agent-loop seat shape
// (direct argv, e.g. ["agy", "--dangerously-skip-permissions"]).
func laneAdapter(lane map[string]any) string {
	command := stringsFromSlice(lane["command"])
	if len(command) == 0 {
		return ""
	}
	base := filepath.Base(strings.TrimSpace(command[0]))
	return strings.TrimSuffix(base, ".exe")
}

func laneDeclaresAgentLoop(lane map[string]any) bool {
	caps, ok := lane["adapter_capabilities"].(map[string]any)
	if !ok {
		return false
	}
	return caps["agent_loop"] == true
}

// laneCommandIsAgyPrint reports whether the lane command invokes the `agy`
// binary with a `--print` (one-shot) flag. It tolerates both a direct argv
// (["agy", "--print", …]) and an `sh -c` stdin shim that execs agy.
// lintDeprecatedClaudePrintLane warns when a claude lane invokes the retired
// one-shot `--print`/`-p` mode. RFC 0088 / D148 retired `-p`/`--print`/`exec` for
// ALL lanes: every lane is now a daemon-owned long-lived interactive PTY
// agent-loop session. `claude --print` cannot run the work-packet loop — under
// the agent-loop executor it prints once and exits without claiming, and as a
// bare lane it never self-claims — so the lane silently parks/dies (the #148
// class). It must not be hardened; it must not be used. Fires regardless of
// adapter_capabilities.agent_loop because `--print` defeats the interactive loop
// even when wrapped.
func lintDeprecatedClaudePrintLane(workflow map[string]any, findings *[]map[string]any) {
	lanes, ok := workflow["lanes"].(map[string]any)
	if !ok {
		return
	}
	for _, laneID := range sortedLaneIDs(lanes) {
		lane, ok := lanes[laneID].(map[string]any)
		if !ok {
			continue
		}
		if !laneCommandIsClaudePrint(lane) {
			continue
		}
		*findings = append(*findings, map[string]any{
			"rule":     "deprecated_claude_print_lane",
			"severity": "warning",
			"message":  "lane '" + laneID + "' runs `claude --print`/`-p`, the retired one-shot mode (RFC 0088 / D148). It cannot run the agent-loop interactive work-packet loop — it prints once and exits without ever claiming, so the lane silently parks/dies (#148). Use a bare interactive command: [\"claude\", \"--dangerously-skip-permissions\"] with \"adapter_capabilities\": {\"agent_loop\": true}",
			"lane_id":  laneID,
		})
	}
}

// laneCommandIsClaudePrint reports whether a lane invokes the `claude` binary
// with a one-shot `--print`/`-p` flag. Uses laneAdapter (basename of command[0])
// so an absolute claude path is still detected.
func laneCommandIsClaudePrint(lane map[string]any) bool {
	if laneAdapter(lane) != "claude" {
		return false
	}
	for _, arg := range stringsFromSlice(lane["command"]) {
		for _, field := range strings.Fields(arg) {
			switch strings.Trim(field, "\"'") {
			case "--print", "-p":
				return true
			}
		}
	}
	return false
}

func laneCommandIsAgyPrint(lane map[string]any) bool {
	command := stringsFromSlice(lane["command"])
	if len(command) == 0 {
		return false
	}
	invokesAgy := false
	usesPrint := false
	for _, arg := range command {
		for _, field := range strings.Fields(arg) {
			switch strings.Trim(field, "\"'") {
			case "agy":
				invokesAgy = true
			case "--print", "-p":
				usesPrint = true
			}
		}
	}
	return invokesAgy && usesPrint
}

func sortedLaneIDs(lanes map[string]any) []string {
	ids := make([]string, 0, len(lanes))
	for laneID := range lanes {
		ids = append(ids, laneID)
	}
	sort.Strings(ids)
	return ids
}

func invalidLintCoverage() map[string]any {
	checks := []map[string]any{
		lintCoverageCheck("reviewer_independence", false, "workflow is invalid; reviewer independence was not evaluated"),
		lintCoverageCheck("fresh_context", false, "workflow is invalid; review context freshness was not evaluated"),
		lintCoverageCheck("write_isolation", false, "workflow is invalid; write isolation was not evaluated"),
		lintCoverageCheck("revision_or_escalation_path", false, "workflow is invalid; revision and escalation paths were not evaluated"),
		lintCoverageCheck("posture_diversity", false, "workflow is invalid; review posture diversity was not evaluated"),
	}
	return map[string]any{"score": 0, "max_score": len(checks), "level": "weak", "checks": checks}
}

func lintCoverage(workflow map[string]any, jobMap map[string]map[string]any, findings []map[string]any) map[string]any {
	rules := map[string]bool{}
	for _, finding := range findings {
		rules[stringValue(finding["rule"])] = true
	}
	reviewJobs := []map[string]any{}
	for _, job := range jobMap {
		if verdictJobTypes[defaultString(job["type"], "generic")] {
			reviewJobs = append(reviewJobs, job)
		}
	}
	reviewerIndependent := !rules["same_model_review_pair"] && !rules["same_model_revision_cycle"] && !rules["same_model_adjudicator_pair"]
	freshContext := !rules["review_without_fresh_context"]
	writeIsolated := !rules["broad_write_scope"] && !rules["repo_write_without_worktree_isolation"]
	hasRevisionOrEscalation := !rules["missing_review_escalation_path"]
	checks := []map[string]any{
		lintCoverageCheck("reviewer_independence", reviewerIndependent, coverageReason(len(reviewJobs) == 0, reviewerIndependent, "workflow has no review jobs", "review lanes are model-family independent", "one or more review lanes share model family with implementation work")),
		lintCoverageCheck("fresh_context", freshContext, coverageReason(len(reviewJobs) == 0, freshContext, "workflow has no review jobs", "review jobs require fresh context", "one or more review jobs can reuse contaminated context")),
		lintCoverageCheck("write_isolation", writeIsolated, coverageReason(false, writeIsolated, "", "repo-write jobs are narrowly scoped and isolated", "one or more repo-write jobs are broad or lack per-job isolation")),
		lintCoverageCheck("revision_or_escalation_path", hasRevisionOrEscalation, coverageReason(len(reviewJobs) == 0, hasRevisionOrEscalation, "workflow has no review jobs", "review verdicts have a revision or human escalation path", "review verdicts lack a revision or human escalation path")),
		lintCoverageCheck("posture_diversity", hasReviewPostureDiversity(reviewJobs), reviewPostureDiversityReason(reviewJobs)),
	}
	score := 0
	for _, check := range checks {
		if check["passed"] == true {
			score++
		}
	}
	return map[string]any{"score": score, "max_score": len(checks), "level": lintCoverageLevel(score, len(checks)), "checks": checks}
}

func lintCoverageCheck(id string, passed bool, reason string) map[string]any {
	return map[string]any{"id": id, "passed": passed, "weight": 1, "reason": reason}
}

func coverageReason(empty bool, passed bool, emptyReason string, passedReason string, failedReason string) string {
	if empty {
		return emptyReason
	}
	if passed {
		return passedReason
	}
	return failedReason
}

func hasReviewPostureDiversity(reviewJobs []map[string]any) bool {
	if len(reviewJobs) == 0 {
		return true
	}
	postures := map[string]bool{}
	for _, job := range reviewJobs {
		posture := stringValue(job["review_posture"])
		if posture == "" {
			posture = "neutral"
		}
		postures[posture] = true
	}
	return len(postures) >= 2
}

func reviewPostureDiversityReason(reviewJobs []map[string]any) string {
	if len(reviewJobs) == 0 {
		return "workflow has no review jobs"
	}
	postures := []string{}
	seen := map[string]bool{}
	for _, job := range reviewJobs {
		posture := stringValue(job["review_posture"])
		if posture == "" {
			posture = "neutral"
		}
		if !seen[posture] {
			seen[posture] = true
			postures = append(postures, posture)
		}
	}
	sort.Strings(postures)
	if len(postures) >= 2 {
		return "review jobs cover multiple postures: " + strings.Join(postures, ", ")
	}
	return "review jobs cover only one posture: " + postures[0]
}

func lintCoverageLevel(score int, maxScore int) string {
	if maxScore <= 0 {
		return "weak"
	}
	if score == maxScore {
		return "strong"
	}
	if score >= max(1, (maxScore*3+4)/5) {
		return "adequate"
	}
	return "weak"
}

func modelFamily(value string) string {
	tokens := []string{}
	for _, token := range modelTokenPattern.Split(strings.ToLower(value), -1) {
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return strings.ToLower(value)
	}
	if (tokens[0] == "openai" || tokens[0] == "anthropic" || tokens[0] == "google") && len(tokens) > 1 {
		return tokens[1]
	}
	return tokens[0]
}

func effectiveFreshSessionRequired(job map[string]any) bool {
	if job["fresh_session_required"] == true {
		return true
	}
	return defaultString(job["type"], "generic") == "review" && job["reviewer_context_policy"] == "fresh" && job["fresh_session_required"] == nil
}

func sortedJobIDs(jobMap map[string]map[string]any) []string {
	ids := make([]string, 0, len(jobMap))
	for jobID := range jobMap {
		ids = append(ids, jobID)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeysFromClaims(values map[string][]sharedResourceClaim) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysFromStringSlices(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
