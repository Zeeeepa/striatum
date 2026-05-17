"""Tests for per-kind front-matter validation in publish-artifact."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


def run_cli(repo: Path, *args: str, check: bool = True) -> dict[str, object]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}"
        )
    if result.stdout.strip() == "":
        return {}
    payload: dict[str, object] = json.loads(result.stdout)
    payload["returncode"] = result.returncode
    return payload


def data(payload: dict[str, object]) -> dict[str, object]:
    value = payload["data"]
    assert isinstance(value, dict)
    return value


def init_repo(repo: Path) -> None:
    run_cli(repo, "init")


def prepare_started_run(repo: Path) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register(repo: Path, run_id: str, role: str, lane: str) -> str:
    args: list[str] = [
        "register-session",
        "--run-id",
        run_id,
        "--role",
        role,
        "--lane",
        lane,
        "--capability",
        "review",
    ]
    if role == "reviewer":
        args += ["--force-non-fresh", "--reason", "test fixture"]
    payload = data(run_cli(repo, *args))
    return str(payload["session_id"])


def claim(repo: Path, session_id: str) -> dict[str, object]:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return packet


def packet_ids(packet: dict[str, object]) -> tuple[str, str, str]:
    job = packet["job"]
    lease = packet["lease"]
    assert isinstance(job, dict)
    assert isinstance(lease, dict)
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def claim_author(repo: Path) -> tuple[str, str, str, str]:
    """Claim the draft job and ack so the author can publish artifacts."""
    run_id = prepare_started_run(repo)
    author = register(repo, run_id, "author", "codex")
    packet = claim(repo, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(repo, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    return author, job_id, lease_id, run_id


def write_artifact(repo: Path, path: str, content: str) -> None:
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def publish(
    repo: Path,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path: str,
    check: bool = True,
) -> dict[str, object]:
    return run_cli(
        repo,
        "publish-artifact",
        "--session-id",
        session_id,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        kind,
        "--logical-name",
        logical_name,
        "--path",
        path,
        check=check,
    )


DRAFT_DIR = "docs/reviews/rfc-ledger"


def test_decision_front_matter_validation_accepts_well_formed_markdown(tmp_path: Path) -> None:
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.decision.v1"\n'
        'artifact_kind: "decision"\n'
        'decision_id: "dec_0001"\n'
        f'run_id: "{run_id}"\n'
        'owner: "human"\n'
        'outcome: "accepted"\n'
        "follow_up_required: false\n"
        'title: "Approve change"\n'
        'created_at: "2026-05-07T00:00:00Z"\n'
        "---\n"
        "\n"
        "Decision body.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/DECISION.md", body)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="decision",
        logical_name="decision",
        path=f"{DRAFT_DIR}/DECISION.md",
    )
    assert result["returncode"] == 0
    assert data(result)["status"] == "published"


def test_decision_front_matter_validation_rejects_bad_outcome(tmp_path: Path) -> None:
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.decision.v1"\n'
        'artifact_kind: "decision"\n'
        'decision_id: "dec_0001"\n'
        f'run_id: "{run_id}"\n'
        'owner: "human"\n'
        'outcome: "bogus"\n'
        "follow_up_required: false\n"
        'title: "Approve change"\n'
        'created_at: "2026-05-07T00:00:00Z"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/DECISION.md", body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="decision",
        logical_name="decision",
        path=f"{DRAFT_DIR}/DECISION.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "outcome" in str(error.get("message", ""))


def test_decision_front_matter_rejects_missing_required_field(tmp_path: Path) -> None:
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.decision.v1"\n'
        'artifact_kind: "decision"\n'
        'decision_id: "dec_0001"\n'
        f'run_id: "{run_id}"\n'
        'owner: "human"\n'
        'outcome: "accepted"\n'
        # missing follow_up_required
        'title: "Approve change"\n'
        'created_at: "2026-05-07T00:00:00Z"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/DECISION.md", body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="decision",
        logical_name="decision",
        path=f"{DRAFT_DIR}/DECISION.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "follow_up_required" in str(error.get("message", ""))


def test_finding_artifact_with_well_formed_front_matter_publishes_cleanly(tmp_path: Path) -> None:
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.finding.v1"\n'
        'artifact_kind: "finding"\n'
        'verdict_intent: "accept_with_findings"\n'
        'severity: "medium"\n'
        'tags: ["scope", "naming"]\n'
        "---\n"
        "\n"
        "Finding body.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/FINDING.md", body)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="finding",
        path=f"{DRAFT_DIR}/FINDING.md",
    )
    assert result["returncode"] == 0
    assert data(result)["status"] == "published"


def test_finding_artifact_rejects_unknown_severity(tmp_path: Path) -> None:
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.finding.v1"\n'
        'artifact_kind: "finding"\n'
        'verdict_intent: "accept"\n'
        'severity: "meh"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/FINDING.md", body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="finding",
        path=f"{DRAFT_DIR}/FINDING.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "severity" in message
    assert "info|low|medium|high|critical" in message


def test_findings_ledger_front_matter_validates_summary_count(tmp_path: Path) -> None:
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.findings_ledger.v1"\n'
        'artifact_kind: "findings_ledger"\n'
        "summary_count: 3\n"
        "---\n"
        "\n"
        "Ledger entries follow as plain Markdown.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/LEDGER.md", body)
    accepted = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="findings_ledger",
        logical_name="ledger",
        path=f"{DRAFT_DIR}/LEDGER.md",
    )
    assert accepted["returncode"] == 0

    bad = (
        "---\n"
        'schema_version: "striatum.findings_ledger.v1"\n'
        'artifact_kind: "findings_ledger"\n'
        "summary_count: -1\n"
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/LEDGER_BAD.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="findings_ledger",
        logical_name="ledger_bad",
        path=f"{DRAFT_DIR}/LEDGER_BAD.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "summary_count" in str(error.get("message", ""))


def test_synthesis_front_matter_inputs_must_be_string_list(tmp_path: Path) -> None:
    author, job_id, lease_id, _ = claim_author(tmp_path)
    bad = (
        "---\n"
        'schema_version: "striatum.synthesis.v1"\n'
        'artifact_kind: "synthesis"\n'
        "inputs: [1, 2]\n"
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/SYNTHESIS.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="synthesis",
        logical_name="synthesis",
        path=f"{DRAFT_DIR}/SYNTHESIS.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "inputs" in str(error.get("message", ""))


def test_artifact_without_front_matter_still_publishes(tmp_path: Path) -> None:
    """Markdown files with no `---` block are accepted unchanged."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    write_artifact(tmp_path, f"{DRAFT_DIR}/PLAIN.md", "Just prose, no front matter.\n")
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="plain_finding",
        path=f"{DRAFT_DIR}/PLAIN.md",
    )
    assert result["returncode"] == 0


def test_non_markdown_artifact_skips_front_matter_validation(tmp_path: Path) -> None:
    """A `.txt` file is not parsed even when content looks like front matter."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    looks_like_front_matter = (
        "---\n"
        "this would be invalid as a finding schema\n"
        "but the publisher must skip non-Markdown files\n"
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/notes.txt", looks_like_front_matter)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="notes_txt",
        path=f"{DRAFT_DIR}/notes.txt",
    )
    assert result["returncode"] == 0


def test_unknown_kind_skips_front_matter_validation(tmp_path: Path) -> None:
    """Kinds with no schema entry remain unschemaed even with a `---` block."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        'random: "value"\n'
        "---\n"
        "\n"
        "Body.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/HANDOFF.md", body)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="handoff",
        logical_name="handoff_with_front_matter",
        path=f"{DRAFT_DIR}/HANDOFF.md",
    )
    assert result["returncode"] == 0


def test_front_matter_unterminated_block_is_rejected(tmp_path: Path) -> None:
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        'schema_version: "striatum.finding.v1"\n'
        'artifact_kind: "finding"\n'
        'verdict_intent: "accept"\n'
        # no closing fence
        "More content but no terminator\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/UNTERMINATED.md", body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="unterminated",
        path=f"{DRAFT_DIR}/UNTERMINATED.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "front matter" in str(error.get("message", ""))


def test_front_matter_unquoted_string_is_rejected(tmp_path: Path) -> None:
    """The minimal parser requires JSON-encoded values; bare strings must error."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "---\n"
        "schema_version: striatum.finding.v1\n"  # bare string, not JSON-quoted
        'artifact_kind: "finding"\n'
        'verdict_intent: "accept"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/BARE.md", body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="finding",
        logical_name="bare_string",
        path=f"{DRAFT_DIR}/BARE.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "JSON" in str(error.get("message", ""))


# --- RFCs 0003 / 0004 / 0005 -----------------------------------------------------

SUPPORT_LEDGER_FLOW_WORKFLOW = ROOT / "examples" / "support-ledger-flow" / "workflow.json"


def _temp_workflow(tmp_path: Path, workflow: dict[str, object]) -> Path:
    path = tmp_path / "workflow.json"
    path.write_text(json.dumps(workflow), encoding="utf-8")
    return path


def _minimal_workflow_for_kind(kind: str, *, path: str) -> dict[str, object]:
    """Build a minimum-shape workflow that declares one expected_artifacts kind."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": f"kind-test-{kind}",
        "workflow_version": "2026-05-07",
        "name": f"Kind validation fixture for {kind}",
        "branch": {
            "mode": "confirm",
            "suggested_name": "striatum/kind-test",
            "allow_dirty": False,
        },
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {
            "codex": {
                "adapter": "process",
                "display_model": "Codex GPT-5.5",
                "command": ["codex", "exec", "--model", "gpt-5.5", "-"],
                "capabilities": ["write", "review"],
            }
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "draft_job",
                "type": "draft",
                "title": "Draft",
                "role_id": "author",
                "lane_id": "codex",
                "objective": "Produce an artifact under test.",
                "task_prompt": {"path": "prompts/draft.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/kind-test/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": kind,
                        "kind": kind,
                        "path": path,
                        "required": True,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def test_artifact_kind_rejects_truly_unknown_kind(tmp_path: Path) -> None:
    """publish-artifact rejects kinds outside ALLOWED_ARTIFACT_KINDS with exit code 6."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    write_artifact(tmp_path, f"{DRAFT_DIR}/BOGUS.md", "Not a real kind.\n")
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="bogus_kind",
        logical_name="bogus",
        path=f"{DRAFT_DIR}/BOGUS.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "bogus_kind" in message
    assert "not in the allowed kinds list" in message


def test_workflow_validation_rejects_unknown_artifact_kind(tmp_path: Path) -> None:
    """validate_workflow rejects expected_artifacts with kinds outside the allowed set."""
    init_repo(tmp_path)
    workflow = _minimal_workflow_for_kind("made_up", path="docs/kind-test/OUT.md")
    rejected = run_cli(
        tmp_path,
        "workflow",
        "validate",
        str(_temp_workflow(tmp_path, workflow)),
        check=False,
    )
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "draft_job" in message
    assert "made_up" in message


def test_workflow_validation_accepts_three_new_kinds(tmp_path: Path) -> None:
    """validate_workflow accepts support_ledger, action_item_ledger, and harness_improvement_proposal."""
    init_repo(tmp_path)
    workflow: dict[str, object] = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "kind-test-three",
        "workflow_version": "2026-05-07",
        "name": "Three new kinds fixture",
        "branch": {
            "mode": "confirm",
            "suggested_name": "striatum/kind-test",
            "allow_dirty": False,
        },
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {
            "codex": {
                "adapter": "process",
                "display_model": "Codex GPT-5.5",
                "command": ["codex", "exec", "--model", "gpt-5.5", "-"],
                "capabilities": ["write", "review"],
            }
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "support",
                "type": "draft",
                "title": "Support ledger",
                "role_id": "author",
                "lane_id": "codex",
                "objective": "Write a support ledger.",
                "task_prompt": {"path": "prompts/support.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/kind-test/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "support_ledger",
                        "kind": "support_ledger",
                        "path": "docs/kind-test/SUPPORT.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "actions",
                "type": "draft",
                "title": "Action item ledger",
                "role_id": "author",
                "lane_id": "codex",
                "objective": "Write an action-item ledger.",
                "task_prompt": {"path": "prompts/actions.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/kind-test/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "action_items",
                        "kind": "action_item_ledger",
                        "path": "docs/kind-test/ACTIONS.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "harness",
                "type": "draft",
                "title": "Harness improvement proposal",
                "role_id": "author",
                "lane_id": "codex",
                "objective": "Write a harness improvement proposal.",
                "task_prompt": {"path": "prompts/harness.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/kind-test/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "harness_proposal",
                        "kind": "harness_improvement_proposal",
                        "path": "docs/kind-test/HARNESS.md",
                        "required": True,
                    }
                ],
            },
        ],
        "edges": [],
        "cycles": [],
    }
    accepted = run_cli(
        tmp_path,
        "workflow",
        "validate",
        str(_temp_workflow(tmp_path, workflow)),
    )
    assert accepted["returncode"] == 0
    assert data(accepted)["valid"] is True


def test_workflow_validation_accepts_escalation_kind(tmp_path: Path) -> None:
    """validate_workflow accepts the RFC 0053 escalation artifact kind."""
    init_repo(tmp_path)
    workflow = _minimal_workflow_for_kind(
        "escalation", path="docs/kind-test/ESCALATION.md"
    )
    accepted = run_cli(
        tmp_path,
        "workflow",
        "validate",
        str(_temp_workflow(tmp_path, workflow)),
    )
    assert accepted["returncode"] == 0
    assert data(accepted)["valid"] is True


def test_escalation_front_matter_validates_human_principal_request(
    tmp_path: Path,
) -> None:
    """A well-formed striatum.escalation.v1 artifact publishes cleanly."""
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    good = (
        "---\n"
        'schema_version: "striatum.escalation.v1"\n'
        'artifact_kind: "escalation"\n'
        'escalation_id: "esc_0001"\n'
        f'run_id: "{run_id}"\n'
        f'job_id: "{job_id}"\n'
        f'session_id: "{author}"\n'
        'severity: "blocked"\n'
        'blocker_kind: "ai_self_declared"\n'
        'description: "Need principal direction before changing scope."\n'
        'reasoning: "Existing decisions do not cover the requested scope expansion."\n'
        'requested_action: "Decide whether to expand the write scope."\n'
        'related_artifacts: ["docs/reviews/rfc-ledger/DRAFT.md"]\n'
        'created_at: "2026-05-17T00:00:00Z"\n'
        "---\n"
        "\n"
        "Escalation body for the principal.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/ESCALATION.md", good)
    accepted = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="escalation",
        logical_name="principal_escalation",
        path=f"{DRAFT_DIR}/ESCALATION.md",
    )
    assert accepted["returncode"] == 0
    assert data(accepted)["status"] == "published"


def test_escalation_front_matter_rejects_unbounded_blocker_kind(
    tmp_path: Path,
) -> None:
    """Escalation artifacts cannot invent new blocker classes ad hoc."""
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    bad = (
        "---\n"
        'schema_version: "striatum.escalation.v1"\n'
        'artifact_kind: "escalation"\n'
        'escalation_id: "esc_0002"\n'
        f'run_id: "{run_id}"\n'
        'severity: "blocked"\n'
        'blocker_kind: "network_is_weird"\n'
        'description: "Need help."\n'
        'reasoning: "The AI operator cannot pick a safe next action."\n'
        'requested_action: "Choose the next action."\n'
        'created_at: "2026-05-17T00:00:00Z"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/ESCALATION_BAD.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="escalation",
        logical_name="bad_principal_escalation",
        path=f"{DRAFT_DIR}/ESCALATION_BAD.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "blocker_kind" in message
    assert "network_is_weird" in message


def test_escalation_front_matter_requires_reasoning(tmp_path: Path) -> None:
    """Escalation artifacts must explain why principal input is needed."""
    author, job_id, lease_id, run_id = claim_author(tmp_path)
    bad = (
        "---\n"
        'schema_version: "striatum.escalation.v1"\n'
        'artifact_kind: "escalation"\n'
        'escalation_id: "esc_0003"\n'
        f'run_id: "{run_id}"\n'
        'severity: "human_checkpoint"\n'
        'blocker_kind: "missing_authority"\n'
        'description: "Authority is missing."\n'
        'requested_action: "Approve or reject the dependency change."\n'
        'created_at: "2026-05-17T00:00:00Z"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/ESCALATION_MISSING_REASONING.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="escalation",
        logical_name="missing_reasoning_escalation",
        path=f"{DRAFT_DIR}/ESCALATION_MISSING_REASONING.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "reasoning" in str(error.get("message", ""))


def test_action_item_ledger_front_matter_validates(tmp_path: Path) -> None:
    """A well-formed striatum.action_item_ledger.v1 publishes; bad revision_round fails."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    good = (
        "---\n"
        'schema_version: "striatum.action_item_ledger.v1"\n'
        'artifact_kind: "action_item_ledger"\n'
        'source_review_artifact: "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md"\n'
        "revision_round: 1\n"
        "total_items: 4\n"
        "---\n"
        "\n"
        "Action items follow as Markdown.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/ACTIONS.md", good)
    accepted = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="action_item_ledger",
        logical_name="action_items",
        path=f"{DRAFT_DIR}/ACTIONS.md",
    )
    assert accepted["returncode"] == 0
    assert data(accepted)["status"] == "published"

    bad = (
        "---\n"
        'schema_version: "striatum.action_item_ledger.v1"\n'
        'artifact_kind: "action_item_ledger"\n'
        'source_review_artifact: "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md"\n'
        'revision_round: "first"\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/ACTIONS_BAD.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="action_item_ledger",
        logical_name="action_items_bad",
        path=f"{DRAFT_DIR}/ACTIONS_BAD.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "revision_round" in str(error.get("message", ""))


def test_harness_improvement_proposal_front_matter_validates(tmp_path: Path) -> None:
    """A well-formed striatum.harness_improvement_proposal.v1 publishes; bad target fails."""
    author, job_id, lease_id, _ = claim_author(tmp_path)
    good = (
        "---\n"
        'schema_version: "striatum.harness_improvement_proposal.v1"\n'
        'artifact_kind: "harness_improvement_proposal"\n'
        'target: "workflow"\n'
        'expected_benefit: "Reduce review cycles for trivial typo fixes."\n'
        'risk: "Low; advisory only."\n'
        'rollback: "Revert the workflow JSON change."\n'
        "---\n"
        "\n"
        "Body of the proposal.\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/HARNESS.md", good)
    accepted = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="harness_improvement_proposal",
        logical_name="harness_proposal",
        path=f"{DRAFT_DIR}/HARNESS.md",
    )
    assert accepted["returncode"] == 0
    assert data(accepted)["status"] == "published"

    bad = (
        "---\n"
        'schema_version: "striatum.harness_improvement_proposal.v1"\n'
        'artifact_kind: "harness_improvement_proposal"\n'
        'target: "infrastructure"\n'
        'expected_benefit: "Speed up CI."\n'
        "---\n"
    )
    write_artifact(tmp_path, f"{DRAFT_DIR}/HARNESS_BAD.md", bad)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="harness_improvement_proposal",
        logical_name="harness_proposal_bad",
        path=f"{DRAFT_DIR}/HARNESS_BAD.md",
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "target" in message
    assert "infrastructure" in message


def test_support_ledger_flow_publishes_three_kinds(tmp_path: Path) -> None:
    """Drive examples/support-ledger-flow/ through produce_artifact and write_support_ledger."""
    init_repo(tmp_path)
    prepared = data(
        run_cli(
            tmp_path,
            "run",
            "prepare",
            "--workflow",
            str(SUPPORT_LEDGER_FLOW_WORKFLOW),
        )
    )
    run_id = str(prepared["run_id"])
    run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test"
    )
    run_cli(tmp_path, "run", "start", "--run-id", run_id)

    # produce_artifact
    author = register(tmp_path, run_id, "author", "codex")
    packet1 = claim(tmp_path, author)
    job_id1, message_id1, lease_id1 = packet_ids(packet1)
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        author,
        "--message-id",
        message_id1,
        "--lease-id",
        lease_id1,
    )
    synthesis_body = (
        "---\n"
        'schema_version: "striatum.synthesis.v1"\n'
        'artifact_kind: "synthesis"\n'
        'inputs: ["produce_artifact"]\n'
        "---\n"
        "\n"
        "Synthesis body for the support-ledger fixture.\n"
    )
    write_artifact(tmp_path, "docs/support-ledger-flow/SYNTHESIS.md", synthesis_body)
    pub1 = publish(
        tmp_path,
        session_id=author,
        job_id=job_id1,
        lease_id=lease_id1,
        kind="synthesis",
        logical_name="synthesis",
        path="docs/support-ledger-flow/SYNTHESIS.md",
    )
    assert pub1["returncode"] == 0
    assert data(pub1)["status"] == "published"
    run_cli(
        tmp_path,
        "complete",
        "--session-id",
        author,
        "--job-id",
        job_id1,
        "--lease-id",
        lease_id1,
    )

    # write_support_ledger
    author2 = register(tmp_path, run_id, "author", "codex")
    packet2 = claim(tmp_path, author2)
    job_id2, message_id2, lease_id2 = packet_ids(packet2)
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        author2,
        "--message-id",
        message_id2,
        "--lease-id",
        lease_id2,
    )
    ledger_body = (
        "---\n"
        'schema_version: "striatum.support_ledger.v1"\n'
        'artifact_kind: "support_ledger"\n'
        'audited_artifact: "docs/support-ledger-flow/SYNTHESIS.md"\n'
        "claim_count: 1\n"
        "---\n"
        "\n"
        "| ID | Claim | Support | Evidence paths | Status |\n"
        "| --- | --- | --- | --- | --- |\n"
        "| SL001 | Synthesis exists. | The file is present in the repo. |"
        " `docs/support-ledger-flow/SYNTHESIS.md` | supported |\n"
    )
    write_artifact(
        tmp_path, "docs/support-ledger-flow/SUPPORT_LEDGER.md", ledger_body
    )
    pub2 = publish(
        tmp_path,
        session_id=author2,
        job_id=job_id2,
        lease_id=lease_id2,
        kind="support_ledger",
        logical_name="support_ledger",
        path="docs/support-ledger-flow/SUPPORT_LEDGER.md",
    )
    assert pub2["returncode"] == 0
    assert data(pub2)["status"] == "published"

    import sqlite3

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        kinds = sorted(
            row[0]
            for row in conn.execute(
                "SELECT artifact_kind FROM artifacts WHERE run_id = ?", (run_id,)
            )
        )
    finally:
        conn.close()
    assert "synthesis" in kinds
    assert "support_ledger" in kinds


def test_synthesis_without_front_matter_auto_attaches_defaults(tmp_path: Path) -> None:
    """Synthesis kind has only-constant required fields; the publisher
    auto-attaches the front matter block when the file omits it, rather
    than refusing or silently dropping the metadata. The file on disk is
    rewritten so the stored SHA and the file content agree downstream.

    The author session in this test is unattested (no supervisor), so the
    expected byline under RFC 0026 attestation rules is
    ``author: operator``.
    """
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body_without_front_matter = (
        "# Synthesis Without Front Matter\n"
        "\n"
        "author: operator\n"
        "\n"
        "## Body\n"
        "\n"
        "Some prose.\n"
    )
    path = f"{DRAFT_DIR}/SYNTHESIS_NO_FM.md"
    write_artifact(tmp_path, path, body_without_front_matter)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="synthesis",
        logical_name="auto_attached_synthesis",
        path=path,
    )
    assert result["returncode"] == 0
    # File on disk now starts with the canonical block.
    final = (tmp_path / path).read_text(encoding="utf-8")
    assert final.startswith(
        '---\nschema_version: "striatum.synthesis.v1"\nartifact_kind: "synthesis"\n---\n'
    )
    # Agent's body is preserved verbatim after the prepended block.
    assert "# Synthesis Without Front Matter" in final
    assert "author: operator" in final
    # Stored author_line reflects the original byline.
    import sqlite3
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT author_line FROM artifacts WHERE logical_name = ?",
            ("auto_attached_synthesis",),
        ).fetchone()
        assert row is not None
        assert row[0] == "author: operator"
    finally:
        conn.close()


def test_bolded_author_line_matches_expected_byline(tmp_path: Path) -> None:
    """A byline written as Markdown bold (``**Author:** value``) is
    recognised by the publisher and stored as the canonical
    ``author: value`` form, rather than being silently dropped or
    rejected as a mismatch. Models seen in dogfood-033 produced this
    decorated form; the canonicaliser normalises it before comparison.

    The author session in this test is unattested (no supervisor), so
    the expected byline value is ``operator``.
    """
    from striatum.artifacts import _canonical_byline_form
    author, job_id, lease_id, _ = claim_author(tmp_path)
    expected_canonical = "author: operator"
    suffix = expected_canonical.split(":", 1)[1].strip()
    bolded = (
        "# Bolded Byline Survives\n"
        "\n"
        f"**Author:** {suffix}\n"
        "\n"
        "## Body\n"
        "Some prose.\n"
    )
    path = f"{DRAFT_DIR}/BOLDED_BYLINE.md"
    write_artifact(tmp_path, path, bolded)
    result = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="handoff",
        logical_name="bolded_byline_handoff",
        path=path,
    )
    assert result["returncode"] == 0
    import sqlite3
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT author_line FROM artifacts WHERE logical_name = ?",
            ("bolded_byline_handoff",),
        ).fetchone()
        assert row is not None
        # Stored canonical form is lowercased ``author: <value>``.
        assert row[0] == expected_canonical
    finally:
        conn.close()
    # And the canonicaliser used internally agrees.
    assert _canonical_byline_form(f"**Author:** {suffix}") == expected_canonical


def test_decorated_author_line_mismatch_still_refuses(tmp_path: Path) -> None:
    """Decoration tolerance must not let a mismatched byline through. A
    decorated byline whose canonical form differs from the expected
    work-packet byline is refused with the documented error.
    """
    author, job_id, lease_id, _ = claim_author(tmp_path)
    body = (
        "# Decorated Wrong Byline\n"
        "\n"
        "**Author:** someone-else-author-0001\n"
        "\n"
        "## Body\n"
    )
    path = f"{DRAFT_DIR}/WRONG_BOLDED.md"
    write_artifact(tmp_path, path, body)
    rejected = publish(
        tmp_path,
        session_id=author,
        job_id=job_id,
        lease_id=lease_id,
        kind="handoff",
        logical_name="wrong_bolded_handoff",
        path=path,
        check=False,
    )
    assert rejected["returncode"] == 6
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "author line must match" in str(error.get("message", ""))
