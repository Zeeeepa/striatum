# Striatum Evidence Export

Run ID: `run_4e95a7c06d1e414cba6765f5045d4d07`
Branch: `striatum/dogfood-034-rfc-0030-0031-rpc-supervision`
Run state: `completed`
Exported at: `2026-05-11T21:54:48Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":9},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-034-rfc-0030-0031-rpc-supervision","run_id":"run_4e95a7c06d1e414cba6765f5045d4d07","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_66bd87643a7a493ebd4d346606441f6d","slug":"designer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_466e661dd7f3490e9d63883cd49d9cab","slug":"designer-claude_code-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_64aa815191b2496d82491ab56de640e1","slug":"designer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"gemini","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_482edf9fe2a240fe86e87d8ee1c74d12","slug":"reviewer-gemini-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"codex","operator_label":null,"pid":"<redacted-free-text>","role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_ba659ec336c9432eae0bd57edfccadc6","slug":"implementer-codex-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_34ddaf10c6524b4f93f0db738ca4ffa8","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":"<redacted-free-text>"},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":null,"lane_id":"claude_code","operator_label":null,"pid":"<redacted-free-text>","role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_3bedaab7204b4836b6eceb5ef102be3b","slug":"reviewer-claude_code-2","state":"closed","supervisor_id":"<redacted-free-text>"}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.23.0' templates_changed=[] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.23.0' templates_changed=['gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.23.0' templates_changed=['codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_33e534505d2d46a5871b2a2d6a3f6fd2",
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
      "content_sha256": "8055d7cb36700e6cc891a05eb012eca714a77ba9ac0178eaff870c40e155ea5e",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/034/BUILD_HANDOFF.md",
      "session_id": "sess_ba659ec336c9432eae0bd57edfccadc6"
    },
    {
      "artifact_id": "art_73ad784ad3aa4bae8d33e79d3ddafa08",
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
      "content_sha256": "a264f3fb9c1a62934b3f389ce6236fb1e9e1b8cff091ac92a680a3b5a7bea447",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement_a2",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/034/BUILD_HANDOFF.md",
      "session_id": "sess_ba659ec336c9432eae0bd57edfccadc6"
    },
    {
      "artifact_id": "art_32e0042b14b749febf05106b1dc8a811",
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
      "content_sha256": "787a6b46a6074ea045aefa456b57d3ceed8c7c73fa32755add9412ffcb6629d8",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/034/DESIGN_SYNTHESIS.md",
      "session_id": "sess_66bd87643a7a493ebd4d346606441f6d"
    },
    {
      "artifact_id": "art_3583f0c734c34fa19522a6989575fcc2",
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
      "content_sha256": "b9716110923ae9f9d19053f50288f46193e359e2a798ce4ad65eecddc6b5b8b7",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/034/design/claude_code/DESIGN.md",
      "session_id": "sess_466e661dd7f3490e9d63883cd49d9cab"
    },
    {
      "artifact_id": "art_522a814069484955a00db8edf13db45c",
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
      "content_sha256": "549a241eda041b51006f7fa36841880e47aeb44c562b0b11f7ce3c7ee1704fb0",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/034/design/codex/DESIGN.md",
      "session_id": "sess_66bd87643a7a493ebd4d346606441f6d"
    },
    {
      "artifact_id": "art_6d3d9e22e84849558951f243677106c8",
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
      "content_sha256": "2cd36b39f1939b83a0b4aa31b9b72b6e297df3a50c7f69115e3804bb46405818",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/034/design/gemini/DESIGN.md",
      "session_id": "sess_64aa815191b2496d82491ab56de640e1"
    },
    {
      "artifact_id": "art_66beaed0b7064fb5a2dc34060ca5be9f",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_threat"
      },
      "content_sha256": "267ff5d4ba542e98d210c0cf4eb6d984e297b058343c60c97a3a596150a67ec5",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat_a2",
      "logical_name": "build_review_threat",
      "repo_path": "docs/dogfood/034/review/build/threat/REVIEW.md",
      "session_id": "sess_3bedaab7204b4836b6eceb5ef102be3b"
    },
    {
      "artifact_id": "art_a4db74564bfd4b96bfa782790f9307a0",
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
      "content_sha256": "6af95d8ccb86d2894be622a54bb00dd96c0a39c6f64cd610b6829739c474c8cd",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat",
      "logical_name": "build_review_threat",
      "repo_path": "docs/dogfood/034/review/build/threat/REVIEW.md",
      "session_id": "sess_34ddaf10c6524b4f93f0db738ca4ffa8"
    },
    {
      "artifact_id": "art_eb8a181576d74500ba665492de967ce1",
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
      "content_sha256": "7403f04692a505bafc3cfe2c593005ac466b17fcd020a24db432e20667517515",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/034/review/design/threat/REVIEW.md",
      "session_id": "sess_482edf9fe2a240fe86e87d8ee1c74d12"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-11T21:54:48Z",
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
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_claude_code",
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
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_codex",
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
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_gemini",
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
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_design_threat",
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
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement"
    },
    {
      "attempt": 2,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement_a2",
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
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_threat"
    },
    {
      "attempt": 2,
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
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_implement_a2",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat_a2",
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
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_design_threat",
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
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-034-rfc-0030-0031-rpc-supervision",
    "run_id": "run_4e95a7c06d1e414cba6765f5045d4d07",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T20:25:37Z",
      "role_id": "designer",
      "session_id": "sess_466e661dd7f3490e9d63883cd49d9cab",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T20:25:37Z",
      "role_id": "designer",
      "session_id": "sess_64aa815191b2496d82491ab56de640e1",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T20:25:37Z",
      "role_id": "designer",
      "session_id": "sess_66bd87643a7a493ebd4d346606441f6d",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T20:45:35Z",
      "role_id": "reviewer",
      "session_id": "sess_482edf9fe2a240fe86e87d8ee1c74d12",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T20:52:45Z",
      "role_id": "implementer",
      "session_id": "sess_ba659ec336c9432eae0bd57edfccadc6",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-11T21:10:22Z",
      "role_id": "reviewer",
      "session_id": "sess_34ddaf10c6524b4f93f0db738ca4ffa8",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-11T21:54:39Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": null,
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-11T21:35:39Z",
      "role_id": "reviewer",
      "session_id": "sess_3bedaab7204b4836b6eceb5ef102be3b",
      "slug": "reviewer-claude_code-2",
      "state": "closed",
      "supervisor_id": "<redacted-free-text>"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_eb8a181576d74500ba665492de967ce1",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_design_threat",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_482edf9fe2a240fe86e87d8ee1c74d12",
      "verdict": "accept",
      "verdict_id": "verdict_f8b677b335f04989bec1ff1272a2a30e"
    },
    {
      "findings_artifact_id": "art_a4db74564bfd4b96bfa782790f9307a0",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_34ddaf10c6524b4f93f0db738ca4ffa8",
      "verdict": "needs_revision",
      "verdict_id": "verdict_e143f510af594b20b1503def68b353ca"
    },
    {
      "findings_artifact_id": "art_66beaed0b7064fb5a2dc34060ca5be9f",
      "job_id": "job_run_4e95a7c06d1e414cba6765f5045d4d07_review_build_threat_a2",
      "posture": "threat_model",
      "rationale": null,
      "session_id": "sess_3bedaab7204b4836b6eceb5ef102be3b",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_48199f3173384561ae0e00c6f6808226"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-034-rfc-0030-0031-rpc-supervision-sealed",
    "workflow_version": "2026-05-11"
  }
}
```
