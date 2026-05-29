package workflowauthoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
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
	lintMissingEscalationPath(workflow, jobMap, &findings)
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
	reviewerIndependent := !rules["same_model_review_pair"] && !rules["same_model_revision_cycle"]
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
