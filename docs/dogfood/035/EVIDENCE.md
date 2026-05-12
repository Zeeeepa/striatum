# Striatum Evidence Export

Run ID: `run_b37634824ee24c8995358cbdcfb11263`
Branch: `striatum/dogfood-035-rfc-0032-cross-repo-mcp-mutation`
Run state: `completed`
Exported at: `2026-05-12T03:50:54Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-035-rfc-0032-cross-repo-mcp-mutation","run_id":"run_b37634824ee24c8995358cbdcfb11263","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_2c01e30a792547d49280d10be334f842","slug":"designer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_91cccdfbde4f469aae9af4e287a893e4","slug":"designer-claude_code-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_6bc0e0ec22164d9bad04671b213ecc60","slug":"designer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_21c6d7936f434902b3f70a0ec812f5f9","slug":"reviewer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_abe2c3033de14979ae4979ba73b2bf38","slug":"implementer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_59e22055483f4938b823747701152e9e","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":"<redacted-free-text>"}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.23.0' templates_changed=['gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.23.0' templates_changed=['codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.23.0' templates_changed=[] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_a0a1492f586b4094965630eba4c98979",
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
      "content_sha256": "924a8d680ac88c2e454808f4b37f5a71e36708a08bc3160d30a8c986dcb529ee",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/035/BUILD_HANDOFF.md",
      "session_id": "sess_abe2c3033de14979ae4979ba73b2bf38"
    },
    {
      "artifact_id": "art_e52af36ea0bb40bbaf7ab01e4d5481bc",
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
      "content_sha256": "7a02254def60a51edbfacb45c7509fa2c6aedc4b95de45409cf6df2cfcfce640",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/035/DESIGN_SYNTHESIS.md",
      "session_id": "sess_2c01e30a792547d49280d10be334f842"
    },
    {
      "artifact_id": "art_7be983dc2b07400eb822a8bd67103d78",
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
      "content_sha256": "eb55ba012c6a077e45e139214f2925965a82f5d50e72554409741dfdb8a00cab",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/035/design/claude_code/DESIGN.md",
      "session_id": "sess_91cccdfbde4f469aae9af4e287a893e4"
    },
    {
      "artifact_id": "art_a65cfac6458344e7afc02c20f95a087b",
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
      "content_sha256": "b0009bfc9725307010811d3a8885536110ff1c98d8c61fc2a2ee0df8fc9c7d8a",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/035/design/codex/DESIGN.md",
      "session_id": "sess_2c01e30a792547d49280d10be334f842"
    },
    {
      "artifact_id": "art_4563992549b14cd08b54d19ddcb52f75",
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
      "content_sha256": "1b64aa9f9ffab6efaed3bc19983b1cd2724264f54b991ac5e23c904495976643",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/035/design/gemini/DESIGN.md",
      "session_id": "sess_6bc0e0ec22164d9bad04671b213ecc60"
    },
    {
      "artifact_id": "art_3311111119f04e3ba547968b78bdb996",
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
      "content_sha256": "8bb084d048b365716a22e27ba54eaade3795be3296c0b2c59d64db58440177fd",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_build_threat",
      "logical_name": "build_review_threat",
      "repo_path": "docs/dogfood/035/review/build/threat/REVIEW.md",
      "session_id": "sess_59e22055483f4938b823747701152e9e"
    },
    {
      "artifact_id": "art_8e8b38c5fe67451996ac0adbc3ab91a3",
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
      "content_sha256": "e9abdd3701136f33052271904e8cb8868ebdd625619180ef5148ca7e664a2b1e",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/035/review/design/threat/REVIEW.md",
      "session_id": "sess_21c6d7936f434902b3f70a0ec812f5f9"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T03:50:54Z",
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
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_claude_code",
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
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_codex",
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
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_gemini",
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
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_design_threat",
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
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_implement",
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
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_build_threat",
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
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_design_threat",
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
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_b37634824ee24c8995358cbdcfb11263_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-035-rfc-0032-cross-repo-mcp-mutation",
    "run_id": "run_b37634824ee24c8995358cbdcfb11263",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T02:44:02Z",
      "role_id": "designer",
      "session_id": "sess_2c01e30a792547d49280d10be334f842",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T02:44:03Z",
      "role_id": "designer",
      "session_id": "sess_6bc0e0ec22164d9bad04671b213ecc60",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T02:44:03Z",
      "role_id": "designer",
      "session_id": "sess_91cccdfbde4f469aae9af4e287a893e4",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:04:36Z",
      "role_id": "reviewer",
      "session_id": "sess_21c6d7936f434902b3f70a0ec812f5f9",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:09:46Z",
      "role_id": "implementer",
      "session_id": "sess_abe2c3033de14979ae4979ba73b2bf38",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T03:50:34Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:39:19Z",
      "role_id": "reviewer",
      "session_id": "sess_59e22055483f4938b823747701152e9e",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_8e8b38c5fe67451996ac0adbc3ab91a3",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_design_threat",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_21c6d7936f434902b3f70a0ec812f5f9",
      "verdict": "accept",
      "verdict_id": "verdict_2ed19a7e9ae34f3eb369d2d0788b2a32"
    },
    {
      "findings_artifact_id": "art_3311111119f04e3ba547968b78bdb996",
      "job_id": "job_run_b37634824ee24c8995358cbdcfb11263_review_build_threat",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_59e22055483f4938b823747701152e9e",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_a8cad469b87743e6a6bf47baf11bafce"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-035-rfc-0032-cross-repo-and-mcp-mutation",
    "workflow_version": "2026-05-12"
  }
}
```
