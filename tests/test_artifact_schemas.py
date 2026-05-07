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
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            role,
            "--lane",
            lane,
            "--capability",
            "review",
        )
    )
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
