package artifactcontracts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Field struct {
	Required bool
	Check    func(any) bool
}

type Schema struct {
	Fields map[string]Field
}

var allowedKinds = map[string]bool{
	"prompt":                       true,
	"finding":                      true,
	"findings_ledger":              true,
	"synthesis":                    true,
	"marker":                       true,
	"handoff":                      true,
	"decision":                     true,
	"patch_summary":                true,
	"test_report":                  true,
	"other":                        true,
	"support_ledger":               true,
	"action_item_ledger":           true,
	"harness_improvement_proposal": true,
	"escalation":                   true,
	"operator_brief":               true,
	"work_plan":                    true,
	"progress_note":                true,
	"operator_report":              true,
	"commit_request":               true,
	"pr_request":                   true,
	"auto_finalize_gate_evidence":  true,
	"collaboration_ledger":         true,
}

var Schemas = map[string]Schema{
	"decision": {
		Fields: map[string]Field{
			"schema_version":     {true, equalsValue("striatum.decision.v1")},
			"artifact_kind":      {true, equalsValue("decision")},
			"decision_id":        {true, isStringValue},
			"run_id":             {true, isStringValue},
			"owner":              {true, equalsValue("human")},
			"outcome":            {true, oneOfValue("accepted", "rejected", "accepted_with_follow_up")},
			"follow_up_required": {true, isBoolValue},
			"title":              {true, isStringValue},
			"created_at":         {true, isStringValue},
			"escape_decision":    {false, isBoolValue},
			"escape_surface":     {false, isNonEmptyStringValue},
			"escape_action":      {false, isNonEmptyStringValue},
		},
	},
	"finding": {
		Fields: map[string]Field{
			"schema_version": {true, equalsValue("striatum.finding.v1")},
			"artifact_kind":  {true, equalsValue("finding")},
			"verdict_intent": {true, oneOfValue("accept", "accept_with_findings", "needs_revision", "reject")},
			"severity":       {false, oneOfValue("info", "low", "medium", "high", "critical")},
			"tags":           {false, isStringListValue},
			// RFC 0098 slice 3: a final_review finding optionally carries a
			// constraint_discharge[] table reporting how each binding constraint
			// landed in the published spec. Additive and optional here; the
			// discharge-verifying gate in pkg/mutations is what makes it
			// mandatory-and-complete for the ACE final-review step.
			"constraint_discharge": {false, isMapListValue},
		},
	},
	"findings_ledger": {
		Fields: map[string]Field{
			"schema_version": {true, equalsValue("striatum.findings_ledger.v1")},
			"artifact_kind":  {true, equalsValue("findings_ledger")},
			"summary_count":  {true, isNonNegativeIntValue},
			"entries_path":   {false, isStringValue},
			// RFC 0098 slice 3: see finding above. A findings_ledger may also
			// carry the discharge table.
			"constraint_discharge": {false, isMapListValue},
		},
	},
	"synthesis": {
		Fields: map[string]Field{
			"schema_version": {true, equalsValue("striatum.synthesis.v1")},
			"artifact_kind":  {true, equalsValue("synthesis")},
			"inputs":         {false, isStringListValue},
		},
	},
	"support_ledger": {
		Fields: map[string]Field{
			"schema_version":   {true, equalsValue("striatum.support_ledger.v1")},
			"artifact_kind":    {true, equalsValue("support_ledger")},
			"audited_artifact": {true, isStringValue},
			"claim_count":      {false, isNonNegativeIntValue},
		},
	},
	"action_item_ledger": {
		Fields: map[string]Field{
			"schema_version":         {true, equalsValue("striatum.action_item_ledger.v1")},
			"artifact_kind":          {true, equalsValue("action_item_ledger")},
			"source_review_artifact": {true, isStringValue},
			"revision_round":         {true, isNonNegativeIntValue},
			"total_items":            {false, isNonNegativeIntValue},
		},
	},
	"harness_improvement_proposal": {
		Fields: map[string]Field{
			"schema_version":   {true, equalsValue("striatum.harness_improvement_proposal.v1")},
			"artifact_kind":    {true, equalsValue("harness_improvement_proposal")},
			"target":           {true, oneOfValue("prompt", "workflow", "spec", "defaults", "documentation")},
			"expected_benefit": {true, isStringValue},
			"risk":             {false, isStringValue},
			"rollback":         {false, isStringValue},
		},
	},
	"escalation": {
		Fields: map[string]Field{
			"schema_version":    {true, equalsValue("striatum.escalation.v1")},
			"artifact_kind":     {true, equalsValue("escalation")},
			"escalation_id":     {true, isNonEmptyStringValue},
			"run_id":            {true, isNonEmptyStringValue},
			"job_id":            {false, isNonEmptyStringValue},
			"session_id":        {false, isNonEmptyStringValue},
			"severity":          {true, oneOfValue("info", "low", "medium", "high", "critical")},
			"blocker_kind":      {true, oneOfValue("ambiguous_goal", "missing_authority", "contradicting_decisions", "no_available_reviewer_lane", "committee_stalemate", "override_required")},
			"description":       {true, isNonEmptyStringValue},
			"reasoning":         {true, isNonEmptyStringValue},
			"requested_action":  {true, isNonEmptyStringValue},
			"related_artifacts": {false, isStringListValue},
			"created_at":        {true, isNonEmptyStringValue},
		},
	},
	"operator_brief": {
		Fields: map[string]Field{
			"schema_version":       {true, equalsValue("striatum.operator_brief.v1")},
			"artifact_kind":        {true, equalsValue("operator_brief")},
			"brief_id":             {true, isNonEmptyStringValue},
			"supersedes":           {true, isNullableNonEmptyStringValue},
			"scope_links":          {true, isScopeLinksValue},
			"context_budget_lines": {true, isNonNegativeIntValue},
			"retrieval_priority":   {true, oneOfValue("high", "medium", "low")},
			"status":               {true, oneOfValue("current", "superseded")},
			"author":               {false, isNonEmptyStringValue},
		},
	},
	"work_plan": {
		Fields: map[string]Field{
			"schema_version":     {true, equalsValue("striatum.work_plan.v1")},
			"artifact_kind":      {true, equalsValue("work_plan")},
			"plan_id":            {true, isNonEmptyStringValue},
			"scope_kind":         {true, oneOfValue("rfc", "phase", "initiative", "bugfix")},
			"scope_ref":          {true, isNonEmptyStringValue},
			"state":              {true, oneOfValue("open", "planned", "in_progress", "closed")},
			"opened_at":          {true, isNonEmptyStringValue},
			"closed_at":          {true, isNullableNonEmptyStringValue},
			"closure_summary":    {true, isNullableNonEmptyStringValue},
			"supersedes":         {true, isNullableNonEmptyStringValue},
			"retrieval_priority": {true, oneOfValue("high", "medium", "low")},
			"author":             {false, isNonEmptyStringValue},
		},
	},
	"progress_note": {
		Fields: map[string]Field{
			"schema_version":     {true, equalsValue("striatum.progress_note.v1")},
			"artifact_kind":      {true, equalsValue("progress_note")},
			"note_date":          {true, isNonEmptyStringValue},
			"session_slug":       {true, isNonEmptyStringValue},
			"related_plan":       {true, isNullableNonEmptyStringValue},
			"related_brief":      {true, isNullableNonEmptyStringValue},
			"retrieval_priority": {true, oneOfValue("high", "medium", "low")},
			"author":             {false, isNonEmptyStringValue},
		},
	},
	"operator_report": {
		Fields: map[string]Field{
			"schema_version":     {true, equalsValue("striatum.operator_report.v1")},
			"artifact_kind":      {true, equalsValue("operator_report")},
			"author":             {false, isNonEmptyStringValue},
			"retrieval_priority": {false, oneOfValue("high", "medium", "low")},
			"supersedes":         {false, isNullableNonEmptyStringValue},
		},
	},
	"commit_request": {
		Fields: map[string]Field{
			"schema_version":      {true, equalsValue("striatum.commit_request.v1")},
			"artifact_kind":       {true, equalsValue("commit_request")},
			"request_id":          {true, isNonEmptyStringValue},
			"run_id":              {false, isNonEmptyStringValue},
			"base_head":           {true, isNonEmptyStringValue},
			"branch":              {true, isNonEmptyStringValue},
			"git_snapshot_hash":   {true, isNonEmptyStringValue},
			"included_paths":      {true, isNonEmptyStringListValue},
			"reviewed_artifacts":  {false, isNonEmptyStringListValue},
			"commit_message":      {true, isNonEmptyStringValue},
			"rationale":           {true, isNonEmptyStringValue},
			"confirmation_status": {true, oneOfValue("pending", "operator_confirmed", "human_confirmed", "refused")},
			"confirmed_by":        {false, isNullableNonEmptyStringValue},
			"confirmed_at":        {false, isNullableNonEmptyStringValue},
		},
	},
	"pr_request": {
		Fields: map[string]Field{
			"schema_version":         {true, equalsValue("striatum.pr_request.v1")},
			"artifact_kind":          {true, equalsValue("pr_request")},
			"request_id":             {true, isNonEmptyStringValue},
			"run_id":                 {false, isNonEmptyStringValue},
			"target_branch":          {true, isNonEmptyStringValue},
			"summary":                {true, isNonEmptyStringValue},
			"body_draft":             {true, isNonEmptyStringValue},
			"related_commit_request": {false, isNullableNonEmptyStringValue},
			"local_commit_sha":       {false, isNullableNonEmptyStringValue},
			"provider_target":        {false, isNullableNonEmptyStringValue},
			"confirmation_status":    {true, oneOfValue("pending", "human_confirmed", "refused")},
			"confirmed_by":           {false, isNullableNonEmptyStringValue},
			"confirmed_at":           {false, isNullableNonEmptyStringValue},
		},
	},
	"auto_finalize_gate_evidence": {
		Fields: map[string]Field{
			"schema_version":               {true, equalsValue("striatum.auto_finalize_gate_evidence.v1")},
			"artifact_kind":                {true, equalsValue("auto_finalize_gate_evidence")},
			"decision_id":                  {true, equalsValue("D125")},
			"gate_status":                  {true, oneOfValue("pending", "satisfied")},
			"live_success_count":           {true, isNonNegativeIntValue},
			"lane_shape_count":             {true, isNonNegativeIntValue},
			"lane_shapes":                  {true, isNonEmptyStringListValue},
			"contested_audit_chain_events": {true, isNonNegativeIntValue},
			"evidence_artifacts":           {true, isNonEmptyStringListValue},
			"created_at":                   {true, isNonEmptyStringValue},
		},
	},
	"collaboration_ledger": {
		Fields: map[string]Field{
			"schema_version": {true, oneOfValue("striatum.collaboration_ledger.v1", "striatum.collaboration_ledger.v1.1")},
			"artifact_kind":  {true, equalsValue("collaboration_ledger")},
			"shape":          {true, oneOfValue("falsification_gate", "cross_examination", "fog_of_war_review", "synaptic_prune", "adjudicated_constraint_extraction")},
			"topic":          {true, isNonEmptyStringValue},
			"participants":   {true, isNonEmptyStringListValue},
			"entries":        {true, isCollaborationLedgerEntriesValue},
			"verdict":        {true, oneOfValue("accept", "accept_with_findings", "needs_revision", "reject")},
			"rationale":      {true, isNonEmptyStringValue},
			"cycle":          {false, isNonNegativeIntValue},
			"constraints":    {false, isMapListValue},
			"branches":       {false, isStringAnyMapValue},
			"findings":       {false, isMapListValue},
		},
	},
}

// StandardOptionalMetadata are byline/workflow metadata keys that any markdown
// artifact may carry without rejection, tolerated free-form (not value-checked).
// Agents naturally include these in artifact templates (author, workflow, phase,
// lane, date, visibility, ...); rejecting them as "unknown fields" forced lanes
// to reverse-engineer each kind's schema mid-run (RFC 0100 / #74 / #79). A kind
// that gives one of these a required, value-checked meaning (e.g. decision.title,
// decision.run_id) still enforces it through its schema Fields — this set only
// suppresses the unknown-field rejection for kinds that do not define the key.
var StandardOptionalMetadata = map[string]bool{
	"author":     true,
	"workflow":   true,
	"phase":      true,
	"lane":       true,
	"role":       true,
	"model":      true,
	"date":       true,
	"created_at": true,
	"updated_at": true,
	"visibility": true,
	"title":      true,
	"status":     true,
	"tags":       true,
	"summary":    true,
	"related":    true,
	"run_id":     true,
	"session_id": true,
	"ordinal":    true,
	"cycle":      true,
}

// SchemaKeyNames returns the sorted required and optional field names a kind's
// schema declares, so validation errors (and, later, work packets) can name the
// contract a lane must satisfy without it reading Go source (RFC 0100).
func SchemaKeyNames(kind string) (required, optional []string, ok bool) {
	schema, ok := Schemas[kind]
	if !ok {
		return nil, nil, false
	}
	for name, field := range schema.Fields {
		if field.Required {
			required = append(required, name)
		} else {
			optional = append(optional, name)
		}
	}
	sort.Strings(required)
	sort.Strings(optional)
	return required, optional, true
}

func standardMetadataNames() []string {
	names := make([]string, 0, len(StandardOptionalMetadata))
	for name := range StandardOptionalMetadata {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// enumFieldValues lists the accepted values for enum front-matter fields so a
// rejection names them (#110 / RFC 0100), instead of a bare "is invalid" that
// forces a lane to read Go source. This map is the error-message source; each
// kind's schema oneOfValue check is the validator — keep the two in sync.
var enumFieldValues = map[string]map[string][]string{
	"collaboration_ledger": {
		"verdict": {"accept", "accept_with_findings", "needs_revision", "reject"},
		"shape":   {"falsification_gate", "cross_examination", "fog_of_war_review", "synaptic_prune", "adjudicated_constraint_extraction"},
	},
	// #126: `severity` was missing here, so `severity: blocker` was rejected with a
	// bare "is invalid" that forced a lane to read Go source for the enum. Keep this
	// map complete with every schema oneOfValue field (TestEnumFieldValuesCoverSchemaEnums
	// guards the sync).
	"finding": {
		"verdict_intent": {"accept", "accept_with_findings", "needs_revision", "reject"},
		"severity":       {"info", "low", "medium", "high", "critical"},
	},
	"decision":                     {"outcome": {"accepted", "rejected", "accepted_with_follow_up"}},
	"commit_request":               {"confirmation_status": {"pending", "operator_confirmed", "human_confirmed", "refused"}},
	"pr_request":                   {"confirmation_status": {"pending", "human_confirmed", "refused"}},
	"harness_improvement_proposal": {"target": {"prompt", "workflow", "spec", "defaults", "documentation"}},
	"escalation": {
		"severity":     {"info", "low", "medium", "high", "critical"},
		"blocker_kind": {"ambiguous_goal", "missing_authority", "contradicting_decisions", "no_available_reviewer_lane", "committee_stalemate", "override_required"},
	},
	"operator_brief": {
		"retrieval_priority": {"high", "medium", "low"},
		"status":             {"current", "superseded"},
	},
	"work_plan": {
		"scope_kind":         {"rfc", "phase", "initiative", "bugfix"},
		"state":              {"open", "planned", "in_progress", "closed"},
		"retrieval_priority": {"high", "medium", "low"},
	},
	"progress_note":               {"retrieval_priority": {"high", "medium", "low"}},
	"operator_report":             {"retrieval_priority": {"high", "medium", "low"}},
	"auto_finalize_gate_evidence": {"gate_status": {"pending", "satisfied"}},
}

// invalidFieldMessage produces an actionable error when a front-matter field
// fails its value check, naming the accepted enum values (or the expected shape)
// so the lane does not have to read Go source (RFC 0100 / #79 / #110).
func invalidFieldMessage(kind, name string) string {
	if fields, ok := enumFieldValues[kind]; ok {
		if values, ok := fields[name]; ok {
			return fmt.Sprintf("%s artifact front matter field %q is invalid; allowed values are %s — see docs/reference/spec.md#artifact-front-matter-schemas", kind, name, strings.Join(values, ", "))
		}
	}
	if kind == "collaboration_ledger" && name == "entries" {
		return `collaboration_ledger artifact front matter field "entries" is invalid; entries must be a non-empty list of objects { by: <participant>, kind: claim|challenge|rebuttal|..., refs: ["dialogue:<seq>", ...] } — see docs/reference/spec.md#artifact-front-matter-schemas`
	}
	return fmt.Sprintf("%s artifact front matter field %q is invalid — see docs/reference/spec.md#artifact-front-matter-schemas", kind, name)
}

// skeletonShapeHints gives a valid YAML example value (and an optional inline
// comment) for non-enum front-matter fields, so Skeleton emits a copy-pasteable
// block. Enum fields are sourced from enumFieldValues; schema_version /
// artifact_kind from convention. A field with no hint falls back to a placeholder.
var skeletonShapeHints = map[string]struct{ value, comment string }{
	"tags":                 {"[]", "list of strings"},
	"constraint_discharge": {"[]", "list of objects"},
	"inputs":               {"[]", "list of upstream artifact refs"},
	"entries":              {"[]", "non-empty list of objects"},
	"summary_count":        {"0", "non-negative integer"},
	"claim_count":          {"0", "non-negative integer"},
	"total_items":          {"0", "non-negative integer"},
	"revision_round":       {"0", "non-negative integer"},
	"follow_up_required":   {"false", "boolean"},
	"created_at":           {"\"2026-01-01T00:00:00Z\"", "RFC3339 timestamp"},
}

// Skeleton returns a minimal, copy-pasteable valid front-matter block for a
// front-matter-bearing artifact kind (#126): every required field with a valid
// example value, then the optional fields (commented), so a rejected lane gets the
// exact shape instead of repairing by trial and error. Returns ok=false for a kind
// with no front-matter schema.
func Skeleton(kind string) (string, bool) {
	req, opt, ok := SchemaKeyNames(kind)
	if !ok {
		return "", false
	}
	var b strings.Builder
	b.WriteString("---\n")
	for _, name := range orderSkeletonFields(req) {
		b.WriteString(skeletonLine(kind, name, false))
	}
	for _, name := range orderSkeletonFields(opt) {
		b.WriteString(skeletonLine(kind, name, true))
	}
	b.WriteString("---")
	return b.String(), true
}

// orderSkeletonFields puts schema_version then artifact_kind first (the
// conventional header), then the remaining fields in their given (sorted) order.
func orderSkeletonFields(names []string) []string {
	rest := make([]string, 0, len(names))
	var hasVersion, hasKind bool
	for _, n := range names {
		switch n {
		case "schema_version":
			hasVersion = true
		case "artifact_kind":
			hasKind = true
		default:
			rest = append(rest, n)
		}
	}
	head := make([]string, 0, 2)
	if hasVersion {
		head = append(head, "schema_version")
	}
	if hasKind {
		head = append(head, "artifact_kind")
	}
	return append(head, rest...)
}

func skeletonLine(kind, name string, optional bool) string {
	value, comment := skeletonValue(kind, name)
	if optional {
		if comment == "" {
			comment = "optional"
		} else {
			comment = "optional; " + comment
		}
	}
	line := name + ": " + value
	if comment != "" {
		line += "   # " + comment
	}
	return line + "\n"
}

func skeletonValue(kind, name string) (value string, comment string) {
	switch name {
	case "schema_version":
		return "striatum." + kind + ".v1", ""
	case "artifact_kind":
		return kind, ""
	}
	if fields, ok := enumFieldValues[kind]; ok {
		if values, ok := fields[name]; ok && len(values) > 0 {
			return values[0], "one of: " + strings.Join(values, ", ")
		}
	}
	if hint, ok := skeletonShapeHints[name]; ok {
		return hint.value, hint.comment
	}
	return "\"<value>\"", ""
}

// skeletonHint renders the minimal valid front-matter block for a rejection
// message (#126), so a lane that hit a front-matter error gets the exact
// copy-pasteable shape in one shot instead of repairing by trial and error.
func skeletonHint(kind string) string {
	if skel, ok := Skeleton(kind); ok {
		return "\n\nminimal valid " + kind + " front matter:\n" + skel
	}
	return ""
}

func IsAllowedKind(kind string) bool {
	return allowedKinds[kind]
}

func AllowedKindSet() map[string]bool {
	result := make(map[string]bool, len(allowedKinds))
	for kind, allowed := range allowedKinds {
		result[kind] = allowed
	}
	return result
}

func SchemaSet() map[string]Schema {
	result := make(map[string]Schema, len(Schemas))
	for kind, schema := range Schemas {
		result[kind] = schema
	}
	return result
}

func HasFrontMatterSchema(kind string) bool {
	_, ok := Schemas[kind]
	return ok
}

func IsMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func EnsureRequiredFrontMatter(kind string, path string, payload []byte) ([]byte, error) {
	if kind != "synthesis" || !IsMarkdownPath(path) {
		return payload, nil
	}
	text := string(payload)
	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		return payload, nil
	}
	prepend := "---\nschema_version: \"striatum.synthesis.v1\"\nartifact_kind: \"synthesis\"\n---\n\n"
	return []byte(prepend + text), nil
}

func ValidateFrontMatter(kind string, path string, payload []byte) error {
	schema, ok := Schemas[kind]
	if !ok || !IsMarkdownPath(path) {
		return nil
	}
	block, ok := FrontMatterBlock(string(payload))
	if !ok {
		if kind == "collaboration_ledger" {
			return fmt.Errorf("%s artifact front matter is required", kind)
		}
		return nil
	}
	parsed, err := ParseFrontMatterBlock(block)
	if err != nil {
		return err
	}
	for name, field := range schema.Fields {
		value, exists := parsed[name]
		if !exists {
			if field.Required {
				req, _, _ := SchemaKeyNames(kind)
				return fmt.Errorf("%s artifact front matter missing required field %q; %s requires [%s] — see docs/reference/spec.md#artifact-front-matter-schemas%s", kind, name, kind, strings.Join(req, ", "), skeletonHint(kind))
			}
			continue
		}
		if !field.Check(value) {
			return fmt.Errorf("%s%s", invalidFieldMessage(kind, name), skeletonHint(kind))
		}
	}
	extra := []string{}
	for name := range parsed {
		if _, ok := schema.Fields[name]; ok {
			continue
		}
		// RFC 0100 / #74 / #79: tolerate natural workflow/byline metadata so a
		// lane is not forced to strip template front matter to publish.
		if StandardOptionalMetadata[name] {
			continue
		}
		extra = append(extra, name)
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		req, opt, _ := SchemaKeyNames(kind)
		return fmt.Errorf("%s artifact front matter has unknown fields: %s; %s accepts required keys [%s], optional keys [%s], plus standard metadata [%s] — see docs/reference/spec.md#artifact-front-matter-schemas%s",
			kind, strings.Join(extra, ", "), kind, strings.Join(req, ", "), strings.Join(opt, ", "), strings.Join(standardMetadataNames(), ", "), skeletonHint(kind))
	}
	if err := validateKindSpecific(kind, path, parsed, payload); err != nil {
		return err
	}
	return nil
}

func ParseAndValidateFrontMatter(kind string, path string, payload []byte) (map[string]any, error) {
	if !IsMarkdownPath(path) {
		return nil, fmt.Errorf("%s artifact must be Markdown to validate front matter", kind)
	}
	block, ok := FrontMatterBlock(string(payload))
	if !ok {
		return nil, fmt.Errorf("%s artifact front matter is required", kind)
	}
	parsed, err := ParseFrontMatterBlock(block)
	if err != nil {
		return nil, err
	}
	if err := ValidateFrontMatter(kind, path, payload); err != nil {
		return nil, err
	}
	return parsed, nil
}

func FrontMatterBlock(text string) (string, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

func ParseFrontMatterBlock(block string) (map[string]any, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(block), &node); err != nil {
		lineNum := 1
		errMsg := err.Error()
		if idx := strings.Index(errMsg, "line "); idx != -1 {
			sub := errMsg[idx+5:]
			if colonIdx := strings.Index(sub, ":"); colonIdx != -1 {
				numStr := sub[:colonIdx]
				if n, parseErr := strconv.Atoi(numStr); parseErr == nil {
					lineNum = n
				}
			}
		}
		markdownLine := lineNum + 1
		return nil, fmt.Errorf("yaml: line %d: syntax error: %s", markdownLine, errMsg)
	}

	if node.Kind == 0 || (node.Kind == yaml.DocumentNode && len(node.Content) == 0) {
		return map[string]any{}, nil
	}

	parsed, err := parseYAMLNode(&node)
	if err != nil {
		return nil, err
	}
	resMap, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("artifact front matter is not a YAML mapping")
	}
	return resMap, nil
}

func parseYAMLNode(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return parseYAMLNode(n.Content[0])
	case yaml.MappingNode:
		result := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			keyNode := n.Content[i]
			valNode := n.Content[i+1]
			key := keyNode.Value
			if _, exists := result[key]; exists {
				return nil, fmt.Errorf("artifact front matter field %q is declared more than once", key)
			}
			val, err := parseYAMLNode(valNode)
			if err != nil {
				return nil, err
			}
			result[key] = val
		}
		return result, nil
	case yaml.SequenceNode:
		result := []any{}
		allStrings := true
		for _, itemNode := range n.Content {
			val, err := parseYAMLNode(itemNode)
			if err != nil {
				return nil, err
			}
			result = append(result, val)
			if _, ok := val.(string); !ok {
				allStrings = false
			}
		}
		if allStrings {
			strResult := make([]string, len(result))
			for i, v := range result {
				strResult[i] = v.(string)
			}
			return strResult, nil
		}
		return result, nil
	case yaml.ScalarNode:
		var val any
		if err := n.Decode(&val); err != nil {
			return nil, err
		}
		return val, nil
	default:
		return nil, nil
	}
}

func ParseFrontMatterValue(value string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "null" || value == "~" {
		return nil, nil
	}
	if strings.HasPrefix(value, "[") {
		var parsed []string
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}
	if strings.HasPrefix(value, `"`) {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed, nil
	}
	return value, nil
}

func validateKindSpecific(kind string, path string, parsed map[string]any, payload []byte) error {
	switch kind {
	case "operator_brief":
		return validateOperatorBrief(parsed, payload)
	case "pr_request":
		if parsed["related_commit_request"] == nil && parsed["local_commit_sha"] == nil {
			return fmt.Errorf("pr_request artifact front matter requires at least one of 'related_commit_request' or 'local_commit_sha'")
		}
	case "auto_finalize_gate_evidence":
		return validateAutoFinalizeGateEvidence(parsed)
	case "collaboration_ledger":
		return validateCollaborationLedger(parsed)
	case "finding", "findings_ledger":
		if _, err := ParseConstraintDischarge(parsed["constraint_discharge"]); err != nil {
			return err
		}
	}
	_ = path
	return nil
}

func validateOperatorBrief(parsed map[string]any, payload []byte) error {
	budget, ok := intValue(parsed["context_budget_lines"])
	if !ok || budget < 0 {
		return nil
	}
	text := strings.ReplaceAll(string(payload), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	bodyStart := 0
	if _, ok := FrontMatterBlock(text); ok {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				bodyStart = i + 1
				break
			}
		}
	}
	bodyLines := 0
	for _, line := range lines[bodyStart:] {
		if strings.TrimSpace(line) != "" {
			bodyLines++
		}
	}
	if bodyLines > budget {
		return fmt.Errorf("operator_brief artifact front matter field 'context_budget_lines' budget exceeded: body has %d lines, limit is %d", bodyLines, budget)
	}
	return nil
}

func validateAutoFinalizeGateEvidence(parsed map[string]any) error {
	if fmt.Sprint(parsed["gate_status"]) != "satisfied" {
		return nil
	}
	liveSuccessCount, _ := intValue(parsed["live_success_count"])
	laneShapeCount, _ := intValue(parsed["lane_shape_count"])
	contestedEvents, _ := intValue(parsed["contested_audit_chain_events"])
	if liveSuccessCount < 3 {
		return fmt.Errorf("auto_finalize_gate_evidence artifact front matter field 'live_success_count' must be at least 3 when gate_status is 'satisfied'")
	}
	if laneShapeCount < 2 {
		return fmt.Errorf("auto_finalize_gate_evidence artifact front matter field 'lane_shape_count' must be at least 2 when gate_status is 'satisfied'")
	}
	if laneShapes, ok := parsed["lane_shapes"].([]string); !ok || len(laneShapes) < 2 {
		return fmt.Errorf("auto_finalize_gate_evidence artifact front matter field 'lane_shapes' must list at least 2 lane shapes when gate_status is 'satisfied'")
	}
	if contestedEvents != 0 {
		return fmt.Errorf("auto_finalize_gate_evidence artifact front matter field 'contested_audit_chain_events' must be 0 when gate_status is 'satisfied'")
	}
	return nil
}

func nonEmptyString(value any) (string, bool) {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

func equalsValue(expected string) func(any) bool {
	return func(value any) bool { return fmt.Sprint(value) == expected }
}

func oneOfValue(options ...string) func(any) bool {
	allowed := map[string]bool{}
	for _, option := range options {
		allowed[option] = true
	}
	return func(value any) bool { return allowed[fmt.Sprint(value)] }
}

func isStringValue(value any) bool {
	_, ok := value.(string)
	return ok
}

func isNonEmptyStringValue(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func isNullableNonEmptyStringValue(value any) bool {
	if value == nil {
		return true
	}
	return isNonEmptyStringValue(value)
}

func isBoolValue(value any) bool {
	_, ok := value.(bool)
	return ok
}

func isNonNegativeIntValue(value any) bool {
	parsed, ok := intValue(value)
	return ok && parsed >= 0
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int64(typed)) {
			return int(typed), true
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

func isStringListValue(value any) bool {
	switch typed := value.(type) {
	case []string:
		return true
	case []any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	}
	return false
}

func isNonEmptyStringListValue(value any) bool {
	values, ok := stringList(value)
	if !ok || len(values) == 0 {
		return false
	}
	for _, item := range values {
		if strings.TrimSpace(item) == "" {
			return false
		}
	}
	return true
}

func isMapListValue(value any) bool {
	_, ok := mapList(value)
	return ok
}

func isStringAnyMapValue(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}

func mapList(value any) ([]map[string]any, bool) {
	switch typed := value.(type) {
	case []map[string]any:
		return typed, true
	case []string:
		if len(typed) == 0 {
			return []map[string]any{}, true
		}
		return nil, false
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			result = append(result, row)
		}
		return result, true
	default:
		return nil, false
	}
}

func isScopeLinksValue(value any) bool {
	values, ok := stringList(value)
	if !ok || len(values) > 5 {
		return false
	}
	for _, item := range values {
		if strings.TrimSpace(item) == "" {
			return false
		}
	}
	return true
}

func stringList(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, text)
		}
		return result, true
	}
	return nil, false
}

func CleanMarkdownPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}
