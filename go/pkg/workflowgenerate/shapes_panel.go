package workflowgenerate

import (
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

func compileImplementationPanel(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, error) {
	if _, err := panelRolePacks(spec); err != nil {
		return nil, nil, nil, err
	}
	proposalCount, err := panelProposalCount(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	scoreDimensions, err := panelScoreDimensions(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	base := spec.ArtifactRoot
	proposalLane := panelProposalLane(spec)
	reviewLane := panelReviewLane(spec, 1)
	dissentLane := panelReviewLane(spec, 2)
	arbitrationLane := panelArbitrationLane(spec)
	decisionLane := panelDecisionLane(spec)
	scorePosture := panelScorePosture(scoreDimensions)
	jobs := []map[string]any{
		job(
			"frame_problem",
			"synthesis",
			"Frame problem",
			"problem_framer",
			proposalLane,
			base,
			"PROBLEM_BRIEF.md",
			"handoff",
			"problem_brief",
			"frame_problem",
			"Publish a concise implementation problem brief with constraints, goals, non-goals, and decision criteria.",
		),
	}
	proposalIDs := []string{}
	scoreIDs := []string{}
	for idx := 0; idx < proposalCount; idx++ {
		suffix := string(rune('a' + idx))
		label := strings.ToUpper(suffix)
		proposalID := "propose_option_" + suffix
		scoreID := "score_option_" + suffix
		proposalIDs = append(proposalIDs, proposalID)
		scoreIDs = append(scoreIDs, scoreID)
		proposal := job(
			proposalID,
			"synthesis",
			"Propose option "+label,
			"proposer_"+suffix,
			proposalLane,
			base+"/proposals/option_"+suffix,
			"PROPOSAL_"+label+".md",
			"handoff",
			"proposal_"+suffix,
			"propose_option",
			"Develop implementation option "+label+" independently from the problem brief.",
		)
		proposal["parallel_group"] = "proposals"
		proposal["fresh_session_required"] = true
		jobs = append(jobs, proposal)
		score := reviewJobForRole(
			scoreID,
			"Score option "+label,
			"scorekeeper",
			reviewLane,
			base+"/scorecards/SCORECARD_"+label+".md",
			scorePosture,
			"scorecard_"+suffix,
			"score_option",
			"Score proposal "+label+" against the implementation-panel dimensions: "+strings.Join(scoreDimensions, ", ")+".",
		)
		score["parallel_group"] = "scorecards"
		jobs = append(jobs, score)
	}
	jobs = append(jobs,
		job(
			"compile_tradeoffs",
			"synthesis",
			"Compile tradeoffs",
			"tradeoff_ledger",
			proposalLane,
			base,
			"TRADEOFF_LEDGER.md",
			"findings_ledger",
			"tradeoff_ledger",
			"compile_tradeoffs",
			"Compile proposal and scorecard evidence into a normalized tradeoff ledger.",
		),
		job(
			"arbitrate",
			"synthesis",
			"Arbitrate preferred option",
			"arbitrator",
			arbitrationLane,
			base,
			"ARBITRATOR_SYNTHESIS.md",
			"synthesis",
			"arbitrator_synthesis",
			"arbitrate",
			"Select or compose the preferred implementation path from the tradeoff ledger and evidence.",
		),
		reviewJobForRole(
			"review_dissent",
			"Review dissent",
			"dissent_reviewer",
			dissentLane,
			base+"/DISSENT_REVIEW.md",
			"devils_advocate",
			"dissent_review",
			"review_dissent",
			"Try to falsify the arbitration before the final decision is recorded.",
		),
		job(
			"record_decision",
			"synthesis",
			"Record decision",
			"principal_decider",
			decisionLane,
			base,
			"DECISION.md",
			"decision",
			"decision",
			"record_decision",
			"Publish the final implementation decision and required follow-up work.",
		),
	)
	edges := []map[string]any{}
	for _, proposalID := range proposalIDs {
		edges = append(edges, map[string]any{"from": "frame_problem", "to": proposalID, "on": "completed"})
	}
	for idx, proposalID := range proposalIDs {
		edges = append(edges, map[string]any{"from": proposalID, "to": scoreIDs[idx], "on": "completed"})
	}
	for _, scoreID := range scoreIDs {
		edges = append(edges, map[string]any{"from": scoreID, "to": "compile_tradeoffs", "on": "completed"})
	}
	edges = append(edges,
		map[string]any{"from": "compile_tradeoffs", "to": "arbitrate", "on": "completed"},
		map[string]any{"from": "arbitrate", "to": "review_dissent", "on": "completed"},
		map[string]any{"from": "review_dissent", "to": "record_decision", "on": "completed"},
	)
	cycles := []map[string]any{
		{
			"from":            "review_dissent",
			"to":              "arbitrate",
			"on_verdict":      "needs_revision",
			"max_iterations":  1,
			"allow_same_lane": true,
		},
	}
	return jobs, edges, cycles, nil
}

func panelRolePacks(spec Spec) ([]string, error) {
	packs, err := panelStringOptionList(spec, "role_packs", "role_pack", []string{"implementation_panel_roles"}, "spec.options.role_packs")
	if err != nil {
		return nil, err
	}
	if err := validateCatalogPacks(packs, "role_pack", "spec.options.role_packs"); err != nil {
		return nil, err
	}
	if _, ok := stringSet(packs)["implementation_panel_roles"]; !ok {
		return nil, genErr("implementation_panel requires implementation_panel_roles", "spec.options.role_packs")
	}
	return packs, nil
}

func panelAdversaryPacks(spec Spec) ([]string, error) {
	packs, err := panelStringOptionList(spec, "adversary_packs", "adversary_pack", []string{"maintainer_cost"}, "spec.options.adversary_packs")
	if err != nil {
		return nil, err
	}
	if err := validateCatalogPacks(packs, "adversary_pack", "spec.options.adversary_packs"); err != nil {
		return nil, err
	}
	return packs, nil
}

func panelStringOptionList(spec Spec, pluralKey, singularKey string, fallback []string, fieldPath string) ([]string, error) {
	raw, ok := spec.Options[pluralKey]
	if !ok {
		raw = spec.Options[singularKey]
	}
	if raw == nil {
		return append([]string(nil), fallback...), nil
	}
	if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
		return []string{text}, nil
	}
	values, err := stringList(raw, fieldPath)
	if err != nil || len(values) == 0 {
		return nil, genErr(strings.TrimPrefix(pluralKey, "panel_")+" must be a non-empty string list", fieldPath)
	}
	return values, nil
}

func validateCatalogPacks(packs []string, kind, fieldPath string) error {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return genErr("workflow template catalog could not be loaded", fieldPath)
	}
	for idx, packID := range packs {
		template, err := catalog.Get(packID)
		if err != nil {
			return genErr(fmt.Sprintf("unknown %s: %s", kind, packID), fmt.Sprintf("%s[%d]", fieldPath, idx))
		}
		if template["kind"] != kind {
			return genErr(fmt.Sprintf("template %q is not a %s", packID, kind), fmt.Sprintf("%s[%d]", fieldPath, idx))
		}
	}
	return nil
}

func panelProposalCount(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["proposal_count"], 3))
	if !ok || value < 2 || value > 3 {
		return 0, genErr("proposal_count must be 2 or 3 for implementation_panel", "spec.options.proposal_count")
	}
	return value, nil
}

func panelScoreDimensions(spec Spec) ([]string, error) {
	values := []string{}
	if raw, ok := spec.Options["score_dimensions"]; ok && raw != nil {
		list, err := stringList(raw, "spec.options.score_dimensions")
		if err != nil || len(list) == 0 {
			return nil, genErr("score_dimensions must be a non-empty string list", "spec.options.score_dimensions")
		}
		for _, item := range list {
			values = append(values, strings.TrimSpace(item))
		}
	}
	if len(values) == 0 {
		packs, err := panelAdversaryPacks(spec)
		if err != nil {
			return nil, err
		}
		catalog, err := workflowtemplates.Load()
		if err != nil {
			return nil, genErr("workflow template catalog could not be loaded", "spec.options.adversary_packs")
		}
		for _, packID := range packs {
			template, err := catalog.Get(packID)
			if err != nil {
				return nil, genErr(fmt.Sprintf("unknown adversary_pack: %s", packID), "spec.options.adversary_packs")
			}
			for _, item := range listFrom(template["postures"]) {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					values = append(values, text)
				}
			}
		}
	}
	if len(values) == 0 {
		values = []string{"maintainability", "migration_risk", "reversibility"}
	}
	for idx, value := range values {
		if !safePanelSlug(value) {
			return nil, genErr("score_dimensions entries must match ^[a-z0-9._-]{1,64}$", fmt.Sprintf("spec.options.score_dimensions[%d]", idx))
		}
	}
	return values, nil
}

func safePanelSlug(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, ch := range value {
		if ch > 127 {
			return false
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func panelScorePosture(scoreDimensions []string) string {
	return "custom:" + scoreDimensions[0]
}

func panelProposalLane(spec Spec) string {
	return authorLane(spec)
}

func panelReviewLane(spec Spec, idx int) string {
	if spec.LaneSet == "multi_review" {
		count := reviewerCount(spec)
		if idx > count {
			idx = count
		}
		return reviewerLane(spec, idx)
	}
	return reviewerLane(spec, 1)
}

func panelArbitrationLane(spec Spec) string {
	return authorLane(spec)
}

func panelDecisionLane(spec Spec) string {
	return authorLane(spec)
}
