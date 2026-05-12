# Striatum Evidence Export

Run ID: `run_9cfd3d8dcee54d8ab4b4338c91893743`
Branch: `striatum/dogfood-036-rfc-0034-workflow-generator`
Run state: `completed`
Exported at: `2026-05-12T05:41:21Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-036-rfc-0034-workflow-generator","run_id":"run_9cfd3d8dcee54d8ab4b4338c91893743","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_5e5a78539fb24382bb3a6216265af38c","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_d82c32e4fe47455d91ce6e90cfee99b3","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_a7db3095c3ba4402a8561afc6bed0b87","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_d71d705a87774b8a8ac224afb2571707","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_109744fc90034268b7f6104c4dac2a70","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_757248f371c94018a71048397503921b","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.24.0' templates_changed=[] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.24.0' templates_changed=['gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.24.0' templates_changed=['codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_ca8b2088225b4b358dc0b45e12b3dee6",
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
      "content_sha256": "ed5576051f64a0de1906b72fc932ea6daea026c29459bf060d356df13bc51ddf",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/036/BUILD_HANDOFF.md",
      "session_id": "sess_109744fc90034268b7f6104c4dac2a70"
    },
    {
      "artifact_id": "art_24963c3a7a9249f1b48bed21fe041cec",
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
      "content_sha256": "e4601555b1eb651c9282603db7bbb8ceca7d08eb7f1fdb93bb2b7baf4d6ac9a9",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/036/DESIGN_SYNTHESIS.md",
      "session_id": "sess_5e5a78539fb24382bb3a6216265af38c"
    },
    {
      "artifact_id": "art_a2e122b3aae142a2bb587a745b21440a",
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
      "content_sha256": "6596dcab69e94387daacb9a2a593c680604029d9c5d621241ab3ea6355b82551",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/036/design/claude_code/DESIGN.md",
      "session_id": "sess_d82c32e4fe47455d91ce6e90cfee99b3"
    },
    {
      "artifact_id": "art_dde65510da274459909bcf1f6f2a3a8b",
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
      "content_sha256": "6b3132f9c0171d9d766e28d3ca3252a84a5e7894033b67a90d7cd3951302a4cb",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/036/design/codex/DESIGN.md",
      "session_id": "sess_5e5a78539fb24382bb3a6216265af38c"
    },
    {
      "artifact_id": "art_f0d5d09dad91471281e39e5065e50ca1",
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
      "content_sha256": "838a03b9f8c11e4e0b485c35f32bc437e6e5abed812b16ba3869a5c6415f6375",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/036/design/gemini/DESIGN.md",
      "session_id": "sess_a7db3095c3ba4402a8561afc6bed0b87"
    },
    {
      "artifact_id": "art_271d8de09692479096464696d04b0814",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_ergonomics"
      },
      "content_sha256": "e85ac54369d6e30fc59ffcc6bcebe46b687ca29c9bddea33e6e3e241014d4d81",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_build_ergonomics",
      "logical_name": "build_review_ergonomics",
      "repo_path": "docs/dogfood/036/review/build/ergonomics/REVIEW.md",
      "session_id": "sess_757248f371c94018a71048397503921b"
    },
    {
      "artifact_id": "art_714940d7251b45649a61267e49b142ea",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: reviewer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_ergonomics"
      },
      "content_sha256": "6bad34651552aa78cae801025605cd20b85a6e0a63f6a25a782b5b3b20e9d932",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_design_ergonomics",
      "logical_name": "design_review_ergonomics",
      "repo_path": "docs/dogfood/036/review/design/ergonomics/REVIEW.md",
      "session_id": "sess_d71d705a87774b8a8ac224afb2571707"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T05:41:21Z",
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
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_claude_code",
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
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_codex",
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
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_gemini",
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
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_design_ergonomics",
          "latest_verdict": "accept",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_ergonomics"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_implement",
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
        "workflow_job_id": "review_build_ergonomics"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_build_ergonomics",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_ergonomics"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_ergonomics"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_design_ergonomics",
      "job_type": "review",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_design_ergonomics"
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
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-036-rfc-0034-workflow-generator",
    "run_id": "run_9cfd3d8dcee54d8ab4b4338c91893743",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:58:55Z",
      "role_id": "designer",
      "session_id": "sess_5e5a78539fb24382bb3a6216265af38c",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:58:55Z",
      "role_id": "designer",
      "session_id": "sess_a7db3095c3ba4402a8561afc6bed0b87",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T03:58:55Z",
      "role_id": "designer",
      "session_id": "sess_d82c32e4fe47455d91ce6e90cfee99b3",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T04:32:10Z",
      "role_id": "reviewer",
      "session_id": "sess_d71d705a87774b8a8ac224afb2571707",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T04:56:46Z",
      "role_id": "implementer",
      "session_id": "sess_109744fc90034268b7f6104c4dac2a70",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T05:40:44Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T05:23:52Z",
      "role_id": "reviewer",
      "session_id": "sess_757248f371c94018a71048397503921b",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_714940d7251b45649a61267e49b142ea",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_design_ergonomics",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_d71d705a87774b8a8ac224afb2571707",
      "verdict": "accept",
      "verdict_id": "verdict_1b559bc1bc7845d48ef2e88e3c784116"
    },
    {
      "findings_artifact_id": "art_271d8de09692479096464696d04b0814",
      "job_id": "job_run_9cfd3d8dcee54d8ab4b4338c91893743_review_build_ergonomics",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_757248f371c94018a71048397503921b",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_6d029f8d9e60480d8670dc94d535923b"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-036-rfc-0034-workflow-generator",
    "workflow_version": "2026-05-12"
  }
}
```
