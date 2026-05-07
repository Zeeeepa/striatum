# Striatum Evidence Export

Run ID: `run_a04880660517480a95438fcc0368d2e0`
Branch: `striatum/dogfood-001-graph-dot`
Run state: `completed`
Exported at: `2026-05-07T22:59:00Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":3},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"runs":[{"branch_name":"striatum/dogfood-001-graph-dot","run_id":"run_a04880660517480a95438fcc0368d2e0","state":"completed"}]}
```

## Doctor Output

```json
{"ok":false,"problems":["active session on terminal run: sess_52019fa306be49e8a37ffb80accc2bac","active session on terminal run: sess_0efae7450216457992a93f18bf4a65c9"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_1953b6d8948a4df0bac0580a01bd15ea",
      "artifact_kind": "handoff",
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "apply_change"
      },
      "content_sha256": "1ff5aa2128881ff2617f7ded4430a71ed49f164271da7de44fd0fd707b547291",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_apply_change",
      "logical_name": "apply_handoff",
      "repo_path": "docs/dogfood/001/APPLY_HANDOFF.md",
      "session_id": "sess_52019fa306be49e8a37ffb80accc2bac"
    },
    {
      "artifact_id": "art_4bdfbe9244ed401db8f2d8031200a50c",
      "artifact_kind": "handoff",
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "content_sha256": "9450c31a0741eeebf7e1bff2633f7fe42a9f4ba1d9a1990e1d7103540879b19b",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_draft_change",
      "logical_name": "draft_handoff",
      "repo_path": "docs/dogfood/001/DRAFT_HANDOFF.md",
      "session_id": "sess_52019fa306be49e8a37ffb80accc2bac"
    },
    {
      "artifact_id": "art_abc0e95ee4fd482bad36fd87146d7f3f",
      "artifact_kind": "harness_improvement_proposal",
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "content_sha256": "e0cbfae1bd85cc1a254c9fac2659c009847781615e41587159ee591be90ba15d",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_draft_change",
      "logical_name": "harness_001",
      "repo_path": "docs/dogfood/001/findings/HARNESS-001.md",
      "session_id": "sess_52019fa306be49e8a37ffb80accc2bac"
    },
    {
      "artifact_id": "art_27a644a886a447c0b232446c9c5263db",
      "artifact_kind": "harness_improvement_proposal",
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "content_sha256": "3ce14da7d3051d98e7b6c209f63004edb9e2c8710e832be9789d8e666084b0d0",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_draft_change",
      "logical_name": "harness_002",
      "repo_path": "docs/dogfood/001/findings/HARNESS-002.md",
      "session_id": "sess_52019fa306be49e8a37ffb80accc2bac"
    },
    {
      "artifact_id": "art_85f99e3ea2f84985b9720e36afd35108",
      "artifact_kind": "finding",
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: reviewer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "content_sha256": "33cb8d74ccd61b823b22195d1751ef2c8d3d48e2edded786709eccc7df9bc9a0",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_review_change",
      "logical_name": "review_finding",
      "repo_path": "docs/dogfood/001/review/FINDING.md",
      "session_id": "sess_0efae7450216457992a93f18bf4a65c9"
    },
    {
      "artifact_id": "art_d4290a8bd7e34279b7b5475ffdd51c34",
      "artifact_kind": "harness_improvement_proposal",
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: reviewer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "content_sha256": "6ee665879381bcb959ece411239c8593f74a3c77288e45bfc1fc326e99272345",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_review_change",
      "logical_name": "harness_004",
      "repo_path": "docs/dogfood/001/review/HARNESS-004.md",
      "session_id": "sess_0efae7450216457992a93f18bf4a65c9"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-07T22:59:00Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "author",
        "workflow_job_id": "apply_change"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_a04880660517480a95438fcc0368d2e0_review_change",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_change"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_apply_change",
      "job_type": "synthesis",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "author",
      "state": "completed",
      "workflow_job_id": "apply_change"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_draft_change",
      "job_type": "draft",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "author",
      "state": "completed",
      "workflow_job_id": "draft_change"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_a04880660517480a95438fcc0368d2e0_draft_change",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "draft_change"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_review_change",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_change"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-001-graph-dot",
    "run_id": "run_a04880660517480a95438fcc0368d2e0",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "verdicts": [
    {
      "findings_artifact_id": "art_85f99e3ea2f84985b9720e36afd35108",
      "job_id": "job_run_a04880660517480a95438fcc0368d2e0_review_change",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_0efae7450216457992a93f18bf4a65c9",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_356936f5d7b94aeeb49ddfdfc3d30d15"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-001-workflow-graph-dot",
    "workflow_version": "2026-05-07"
  }
}
```
