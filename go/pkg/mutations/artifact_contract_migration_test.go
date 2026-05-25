package mutations

import (
	"strings"
	"testing"
)

func TestLedgerFrontMatterSchemasRemainStrict(t *testing.T) {
	cases := []struct {
		name string
		kind string
		body string
	}{
		{
			name: "support-ledger",
			kind: "support_ledger",
			body: `---
schema_version: "striatum.support_ledger.v1"
artifact_kind: "support_ledger"
audited_artifact: "docs/reviews/REVIEW.md"
claim_count: 2
---

# Support Ledger
`,
		},
		{
			name: "action-item-ledger",
			kind: "action_item_ledger",
			body: `---
schema_version: "striatum.action_item_ledger.v1"
artifact_kind: "action_item_ledger"
source_review_artifact: "docs/reviews/REVIEW.md"
revision_round: 1
total_items: 3
---

# Action Items
`,
		},
		{
			name: "harness-improvement",
			kind: "harness_improvement_proposal",
			body: `---
schema_version: "striatum.harness_improvement_proposal.v1"
artifact_kind: "harness_improvement_proposal"
target: "workflow"
expected_benefit: "clearer operator gate"
risk: "low"
rollback: "remove proposal"
---

# Harness Improvement
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateArtifactFrontMatter(tc.kind, "artifact.md", []byte(tc.body)); err != nil {
				t.Fatalf("valid %s front matter refused: %v", tc.kind, err)
			}
		})
	}
}

func TestLedgerFrontMatterRejectsUnknownFields(t *testing.T) {
	payload := []byte(`---
schema_version: "striatum.support_ledger.v1"
artifact_kind: "support_ledger"
audited_artifact: "docs/reviews/REVIEW.md"
private_note: "must not pass"
---

# Support Ledger
`)
	err := validateArtifactFrontMatter("support_ledger", "SUPPORT.md", payload)
	if err == nil {
		t.Fatal("unknown front matter field should be refused")
	}
	if !strings.Contains(err.Error(), "unknown fields") {
		t.Fatalf("error = %q", err.Error())
	}
}
