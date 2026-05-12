# Striatum Evidence Export

Run ID: `run_7b4f5c0614264a96b00cc923b6741ee5`
Branch: `striatum/dogfood-038-rfc-0036-mcp-harness`
Run state: `completed`
Exported at: `2026-05-12T09:11:04Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-038-rfc-0036-mcp-harness","run_id":"run_7b4f5c0614264a96b00cc923b6741ee5","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_308be92bfba1489faa834e01e0b8a2b2","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_a10a8310974d4a10b04029ecd534ddaf","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_6c77463137cc41ba808fa2ac0236d8ac","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_9b085caaabdb48598583810f943ae467","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_31944c983fe84691a53ffd04122fb667","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_75c5189be55f47e2b2f62b1600297a99","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.25.0' templates_changed=['gemini/GEMINI.md.tmpl', 'gemini/README.md.tmpl', 'gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.25.0' templates_changed=['codex/README.md.tmpl', 'codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.25.0' templates_changed=['claude_code/README.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_716a58d4f7ce41ff9006bb020205ff07",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement"
      },
      "content_sha256": "b93f4c08c6f883e2ada102ac94d1cda5d0a9abd0c00b5fc1b3b952ca90f7e743",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/038/BUILD_HANDOFF.md",
      "session_id": "sess_31944c983fe84691a53ffd04122fb667"
    },
    {
      "artifact_id": "art_33e1661decf44b5eb80dc19d516b28c8",
      "artifact_kind": "synthesis",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "synthesize_design"
      },
      "content_sha256": "db5104cdbf7d577543f4bb8779893cbde5d3e7d7cbceb6a8d70f732a9fe31dbb",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/038/DESIGN_SYNTHESIS.md",
      "session_id": "sess_308be92bfba1489faa834e01e0b8a2b2"
    },
    {
      "artifact_id": "art_cacd3ec8c93740dcae4944d15480d05c",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: designer-claude-opus-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_claude_code"
      },
      "content_sha256": "775be08785c0a87ff2ae11679779da9d4bfa336604e25aa2528d7b4c475d8c32",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/038/design/claude_code/DESIGN.md",
      "session_id": "sess_a10a8310974d4a10b04029ecd534ddaf"
    },
    {
      "artifact_id": "art_3742904257c246efa8cc1e5674d0fd55",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_codex"
      },
      "content_sha256": "10b503506c852fc59183c53f3fda8386bb78b9b3d38bebb3571677b988a84ddd",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/038/design/codex/DESIGN.md",
      "session_id": "sess_308be92bfba1489faa834e01e0b8a2b2"
    },
    {
      "artifact_id": "art_284c8c0506974afa975946ca1aa9c610",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: designer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_gemini"
      },
      "content_sha256": "259629d691d383d9eeebfce09043d8ff35b158bf659f8958492f2cdfd36497e5",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/038/design/gemini/DESIGN.md",
      "session_id": "sess_6c77463137cc41ba808fa2ac0236d8ac"
    },
    {
      "artifact_id": "art_3baef52593e741ca89f5bb6bb55b8541",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_threat"
      },
      "content_sha256": "d68928c3ce893243f93aa0e9fc9475ff0691b7ca1bdde8b33b536447bb56b344",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_build_threat",
      "logical_name": "build_review_threat",
      "repo_path": "docs/dogfood/038/review/build/threat/REVIEW.md",
      "session_id": "sess_75c5189be55f47e2b2f62b1600297a99"
    },
    {
      "artifact_id": "art_e28ec6d39eca459ab3cab85376422e08",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: reviewer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_threat"
      },
      "content_sha256": "c2f3f594c72d4dfd2b5c42c4c25f8acde744b7994f1e44f6728897c9a7607d29",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/038/review/design/threat/REVIEW.md",
      "session_id": "sess_9b085caaabdb48598583810f943ae467"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T09:11:04Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_claude_code"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_claude_code",
      "job_type": "generic",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_claude_code"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_codex",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_codex"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_gemini"
      },
      "dependencies": [],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_gemini",
      "job_type": "generic",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_gemini"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_design_threat",
          "latest_verdict": "accept",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_threat"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_implement",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_threat"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_build_threat",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_threat"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_threat"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_design_threat",
      "job_type": "review",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_design_threat"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "synthesize_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-038-rfc-0036-mcp-harness",
    "run_id": "run_7b4f5c0614264a96b00cc923b6741ee5",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T07:06:54Z",
      "role_id": "designer",
      "session_id": "sess_308be92bfba1489faa834e01e0b8a2b2",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T07:06:54Z",
      "role_id": "designer",
      "session_id": "sess_6c77463137cc41ba808fa2ac0236d8ac",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T07:06:54Z",
      "role_id": "designer",
      "session_id": "sess_a10a8310974d4a10b04029ecd534ddaf",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T07:35:09Z",
      "role_id": "reviewer",
      "session_id": "sess_9b085caaabdb48598583810f943ae467",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T07:46:55Z",
      "role_id": "implementer",
      "session_id": "sess_31944c983fe84691a53ffd04122fb667",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T09:10:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T08:53:39Z",
      "role_id": "reviewer",
      "session_id": "sess_75c5189be55f47e2b2f62b1600297a99",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_e28ec6d39eca459ab3cab85376422e08",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_design_threat",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_9b085caaabdb48598583810f943ae467",
      "verdict": "accept",
      "verdict_id": "verdict_8bf6619ae8374b71ae24fa105e6a1c69"
    },
    {
      "findings_artifact_id": "art_3baef52593e741ca89f5bb6bb55b8541",
      "job_id": "job_run_7b4f5c0614264a96b00cc923b6741ee5_review_build_threat",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_75c5189be55f47e2b2f62b1600297a99",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_94047fcfc655461b8e8034f2e5e8c55b"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-038-rfc-0036-mcp-harness",
    "workflow_version": "2026-05-13"
  }
}
```
