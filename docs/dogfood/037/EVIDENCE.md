# Striatum Evidence Export

Run ID: `run_7e2416bf54d84379ab63a6d141e517bd`
Branch: `striatum/dogfood-037-rfc-0035-multi-repo-test-harness`
Run state: `completed`
Exported at: `2026-05-12T11:18:01Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-037-rfc-0035-multi-repo-test-harness","run_id":"run_7e2416bf54d84379ab63a6d141e517bd","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_1e1b0c2fd0e9468b9c2973ccb96e8752","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_9f355b29ddd848999a62694bbdfc4859","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_78d807d86b154699a63a1b35a39e6695","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_406ac4ba61a4419bb4ecbe039f732f0a","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_5a9bad1218d348d8a4c3f87496e0f252","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_463939852fd246aba7905e6cbb25eed3","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.26.0' templates_changed=['codex/README.md.tmpl', 'codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`","plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.26.0' templates_changed=['gemini/GEMINI.md.tmpl', 'gemini/README.md.tmpl', 'gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.26.0' templates_changed=['claude_code/README.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_8b41be435b714cdc841a33734d156e2c",
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
      "content_sha256": "88f5fb24ba1bd3b35ddd80d1c5910a492c82eb8faca0516570bd24acbeaa2deb",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/037/BUILD_HANDOFF.md",
      "session_id": "sess_5a9bad1218d348d8a4c3f87496e0f252"
    },
    {
      "artifact_id": "art_b5668edeb0e949959e256050a4a1775f",
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
      "content_sha256": "3acc6460ccce119a6d7548a381b8a79b2d0541c3e9eda688b2e64d5ae8830de0",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/037/DESIGN_SYNTHESIS.md",
      "session_id": "sess_1e1b0c2fd0e9468b9c2973ccb96e8752"
    },
    {
      "artifact_id": "art_0ea1998981664fc4b8603171ba1990e6",
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
      "content_sha256": "7598a370fa5d113f65670b3c68e317b7c00e38bd9e36050aa62f87c7753b75c5",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/037/design/claude_code/DESIGN.md",
      "session_id": "sess_9f355b29ddd848999a62694bbdfc4859"
    },
    {
      "artifact_id": "art_0ac93231201b412995e826fec356a29d",
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
      "content_sha256": "8546a3f02ab575e38d883acf8d9f244a8d813125af6a994cb072b28db6de509f",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/037/design/codex/DESIGN.md",
      "session_id": "sess_1e1b0c2fd0e9468b9c2973ccb96e8752"
    },
    {
      "artifact_id": "art_85552f2ff248435cbf8baf44061b1522",
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
      "content_sha256": "682f6b68b03bec90bcb1ed7ba57e609ad100b129e4c18698e9b0fc9176b987df",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/037/design/gemini/DESIGN.md",
      "session_id": "sess_78d807d86b154699a63a1b35a39e6695"
    },
    {
      "artifact_id": "art_4eef2f8d7a084850b2241dedb8d2edb9",
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
      "content_sha256": "4b80a061e585bfa5e52ae65f488c8c8160fc830b6daac40e60290923d04fc78d",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_build_threat",
      "logical_name": "build_review_threat",
      "repo_path": "docs/dogfood/037/review/build/threat/REVIEW.md",
      "session_id": "sess_463939852fd246aba7905e6cbb25eed3"
    },
    {
      "artifact_id": "art_a1839e313d1e42ffa5383cdac42bede7",
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
      "content_sha256": "0b4bd0514d72a963812078aedde88f2829780b2e50a06cebc0f104abf0351640",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/037/review/design/threat/REVIEW.md",
      "session_id": "sess_406ac4ba61a4419bb4ecbe039f732f0a"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T11:18:01Z",
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
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_claude_code",
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
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_codex",
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
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_gemini",
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
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_design_threat",
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
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_implement",
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
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_build_threat",
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
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_design_threat",
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
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-037-rfc-0035-multi-repo-test-harness",
    "run_id": "run_7e2416bf54d84379ab63a6d141e517bd",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T09:33:44Z",
      "role_id": "designer",
      "session_id": "sess_1e1b0c2fd0e9468b9c2973ccb96e8752",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T09:33:44Z",
      "role_id": "designer",
      "session_id": "sess_78d807d86b154699a63a1b35a39e6695",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T09:33:44Z",
      "role_id": "designer",
      "session_id": "sess_9f355b29ddd848999a62694bbdfc4859",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T10:02:22Z",
      "role_id": "reviewer",
      "session_id": "sess_406ac4ba61a4419bb4ecbe039f732f0a",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T10:13:46Z",
      "role_id": "implementer",
      "session_id": "sess_5a9bad1218d348d8a4c3f87496e0f252",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T11:17:49Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T10:59:46Z",
      "role_id": "reviewer",
      "session_id": "sess_463939852fd246aba7905e6cbb25eed3",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_a1839e313d1e42ffa5383cdac42bede7",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_design_threat",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_406ac4ba61a4419bb4ecbe039f732f0a",
      "verdict": "accept",
      "verdict_id": "verdict_7901983975054a97aab3153392b0407d"
    },
    {
      "findings_artifact_id": "art_4eef2f8d7a084850b2241dedb8d2edb9",
      "job_id": "job_run_7e2416bf54d84379ab63a6d141e517bd_review_build_threat",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_463939852fd246aba7905e6cbb25eed3",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_6d8157a706834cdea3888702c6f28a1c"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-037-rfc-0035-multi-repo-test-harness",
    "workflow_version": "2026-05-13"
  }
}
```
