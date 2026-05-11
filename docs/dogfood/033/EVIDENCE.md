# Striatum Evidence Export

Run ID: `run_95475e5eff0247908c0bd6d23c5c6200`
Branch: `striatum/dogfood-033-rfc-0033-substrate`
Run state: `completed`
Exported at: `2026-05-11T17:55:33Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":6},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-033-rfc-0033-substrate","run_id":"run_95475e5eff0247908c0bd6d23c5c6200","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_dfe41dc66df34193bce77b8523d105a0","slug":"designer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_f41e5f31a3f246a687233fd0211993fc","slug":"designer-claude_code-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_4e009a7db4744194a235bcbb6bec2f44","slug":"designer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_b9329af0828d49d1a9412f132f73a5f6","slug":"reviewer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_3d0967091b694f31a049666253d496ad","slug":"implementer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.21.1' templates_changed=['gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.21.1' templates_changed=[] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.21.1' templates_changed=['codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_8e0a3480f97d469ca0388f271265b6dc",
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
      "content_sha256": "cb8031213d2ac69012de5b36f66deddae34305190397cf1e78f0e3c8eb2824a1",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/033/BUILD_HANDOFF.md",
      "session_id": "sess_3d0967091b694f31a049666253d496ad"
    },
    {
      "artifact_id": "art_eef1ca7922b941959a77b300d1e04f53",
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
      "content_sha256": "63b418767886c1d1153fbb47765ee9f4fe50c745698fbe4aa583f5cf0281f89b",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/033/DESIGN_SYNTHESIS.md",
      "session_id": "sess_dfe41dc66df34193bce77b8523d105a0"
    },
    {
      "artifact_id": "art_027e463036bb402284d8c894b13d99dd",
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
      "content_sha256": "e9cb7f92f476a7bdbbcef2a3f7e8c0a66d4dc5964dd673fb944034fbb89e2490",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/033/design/claude_code/DESIGN.md",
      "session_id": "sess_f41e5f31a3f246a687233fd0211993fc"
    },
    {
      "artifact_id": "art_20e069ef6bf84d5e852ea97afd92a247",
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
      "content_sha256": "849ffb0caf93d5c78e52c35cbe699bf3b7b369f72fa8c47d7772f39e81fbf178",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/033/design/codex/DESIGN.md",
      "session_id": "sess_dfe41dc66df34193bce77b8523d105a0"
    },
    {
      "artifact_id": "art_3a52b668b4dc4fafa7f784b44cfb3834",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": null,
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_gemini"
      },
      "content_sha256": "8304b2bddb721e63f3da0279ad08d981e062a2a5ecb674473e7d3a7dc258a8bf",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/033/design/gemini/DESIGN.md",
      "session_id": "sess_4e009a7db4744194a235bcbb6bec2f44"
    },
    {
      "artifact_id": "art_97fd184f0f6c4ba6bfc5f405a34c9422",
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
      "content_sha256": "b9cdc467977a194a9900285da1c35585737585e5543d1649c4ab6c4ee017fdbd",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/033/review/design/threat/REVIEW.md",
      "session_id": "sess_b9329af0828d49d1a9412f132f73a5f6"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-11T17:55:33Z",
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
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_claude_code",
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
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_codex",
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
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_gemini",
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
          "depends_on_job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_review_design_threat",
          "latest_verdict": "accept_with_findings",
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
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_implement",
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
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_threat"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_review_design_threat",
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
          "depends_on_job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-033-rfc-0033-substrate",
    "run_id": "run_95475e5eff0247908c0bd6d23c5c6200",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T17:54:18Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T17:10:43Z",
      "role_id": "designer",
      "session_id": "sess_4e009a7db4744194a235bcbb6bec2f44",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T17:54:18Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T17:10:43Z",
      "role_id": "designer",
      "session_id": "sess_dfe41dc66df34193bce77b8523d105a0",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T17:54:18Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T17:10:43Z",
      "role_id": "designer",
      "session_id": "sess_f41e5f31a3f246a687233fd0211993fc",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T17:54:18Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T17:32:37Z",
      "role_id": "reviewer",
      "session_id": "sess_b9329af0828d49d1a9412f132f73a5f6",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T17:54:18Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T17:40:06Z",
      "role_id": "implementer",
      "session_id": "sess_3d0967091b694f31a049666253d496ad",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_97fd184f0f6c4ba6bfc5f405a34c9422",
      "job_id": "job_run_95475e5eff0247908c0bd6d23c5c6200_review_design_threat",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_b9329af0828d49d1a9412f132f73a5f6",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_9293d2349eda4d3893cbd79b6cbe2d9f"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-033-rfc-0033-storage-substrate",
    "workflow_version": "2026-05-11"
  }
}
```
