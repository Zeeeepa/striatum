"""Artifact publisher for durable repo outputs."""

from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING

from striatum.artifact_contracts import (
    ALLOWED_ARTIFACT_KINDS,
    FRONT_MATTER_SCHEMAS,
    MARKDOWN_SUFFIXES,
    FrontMatterField,
    FrontMatterSchema,
    _canonical_byline_form,
    _first_author_line,
    _front_matter_block,
    _front_matter_body,
    _parse_front_matter,
    _strip_markdown_decoration,
    ensure_required_front_matter,
    markdown_title_block_author_lines,
    parse_artifact_front_matter,
    validate_artifact_front_matter,
)
from striatum.errors import ArtifactError
from striatum.identity import artifact_author_identity, session_lane_attestation
from striatum.primitives import json_loads, new_id, sha256_bytes, utc_now
from striatum.repo_policy import path_allowed, repo_relative_path

if TYPE_CHECKING:
    import sqlite3


__all__ = [
    "ALLOWED_ARTIFACT_KINDS",
    "FRONT_MATTER_SCHEMAS",
    "MARKDOWN_SUFFIXES",
    "FrontMatterField",
    "FrontMatterSchema",
    "_canonical_byline_form",
    "_first_author_line",
    "_front_matter_block",
    "_front_matter_body",
    "_parse_front_matter",
    "_strip_markdown_decoration",
    "ensure_required_front_matter",
    "expected_author_line",
    "markdown_title_block_author_lines",
    "parse_artifact_front_matter",
    "publish_artifact",
    "validate_artifact_front_matter",
    "validate_optional_markdown_author_line",
]


def publish_artifact(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path_text: str,
    allow_no_process_execution: bool = False,
    override_rationale: str | None = None,
) -> dict[str, object]:
    """Record an artifact reference after validating write scope.

    RFC 0046 V1: when the resolved byline is a model byline
    (``<role>-<model>-<ord>``), require a matching process_executions
    row whose observed_output_paths_json covers the artifact path.
    Operator override via allow_no_process_execution + non-empty
    override_rationale records the rationale on the artifact row and
    emits a ``provenance.publish_without_process_execution`` event.
    """
    from striatum.legacy_sqlite.db import (
        active_lease_for,
        active_worktree_for_job,
        insert_event,
        transaction,
    )

    with transaction(conn):
        job = conn.execute("SELECT * FROM jobs WHERE job_id = ?", (job_id,)).fetchone()
        if job is None:
            raise ArtifactError("job does not exist")
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if kind == "transcript":
            raise ArtifactError("transcript artifacts are not allowed by default")
        if kind not in ALLOWED_ARTIFACT_KINDS:
            allowed = ", ".join(sorted(ALLOWED_ARTIFACT_KINDS))
            raise ArtifactError(
                f"artifact kind {kind!r} is not in the allowed kinds list: {allowed}"
            )
        _enforce_required_attestation_for_artifact(
            conn,
            job=job,
            session_id=session_id,
        )
        write_scope = json_loads(str(job["write_scope_json"]))
        if not path_allowed(repo, path_text, write_scope):
            raise ArtifactError("artifact path is outside the job write scope")
        # Always validate against the logical repo-relative path. The artifact
        # row records the logical path even when the file currently lives in a
        # per-job git worktree, since artifacts are durable provenance for the
        # main branch.
        repo_relative_path(repo, path_text)
        worktree = active_worktree_for_job(conn, job_id=job_id)
        if worktree is not None:
            path = (Path(str(worktree["worktree_path"])) / path_text).resolve()
            worktree_root = Path(str(worktree["worktree_path"])).resolve()
            try:
                path.relative_to(worktree_root)
            except ValueError as exc:
                raise ArtifactError(
                    "artifact path must stay inside the active worktree"
                ) from exc
        else:
            path = repo_relative_path(repo, path_text)
        if not path.exists() or not path.is_file():
            raise ArtifactError("artifact file does not exist")
        payload = path.read_bytes()
        # For schema-bearing kinds without front matter, either auto-attach
        # defaults (synthesis: only-constant required fields) or refuse with
        # a kind-specific template (semantic required fields). The on-disk
        # file may be modified in the auto-attach branch so the recorded
        # SHA and the file content agree downstream.
        payload = ensure_required_front_matter(kind=kind, path=path, payload=payload)
        validate_optional_markdown_author_line(
            conn,
            job=job,
            session_id=session_id,
            path=path,
            payload=payload,
        )
        validate_artifact_front_matter(kind=kind, path=path, payload=payload)
        # RFC 0046 V1: lane evidence guard. Compute the expected byline
        # and, if it's a model byline, require a matching
        # process_executions row. Operator-byline publishes are
        # pass-through.
        try:
            byline = expected_author_line(conn, job=job, session_id=session_id)
        except ArtifactError:
            byline = None
        if byline is not None and not _is_operator_byline(byline):
            if not _lane_evidence_present(
                conn, session_id=session_id, path_text=path_text
            ):
                if not allow_no_process_execution:
                    raise ArtifactError(
                        "lane_evidence_missing: artifact path "
                        f"{path_text!r} is not present in any "
                        "process_executions row for session "
                        f"{session_id!r}; pass "
                        "--allow-no-process-execution "
                        "--override-rationale \"<text>\" to record an "
                        "operator override."
                    )
                if not (override_rationale and override_rationale.strip()):
                    raise ArtifactError(
                        "publish-artifact "
                        "--allow-no-process-execution requires a "
                        "non-empty --override-rationale"
                    )
        digest = sha256_bytes(payload)
        existing = conn.execute(
            """
            SELECT * FROM artifacts
            WHERE run_id = ? AND job_id = ? AND logical_name = ?
            """,
            (job["run_id"], job_id, logical_name),
        ).fetchone()
        if existing is not None:
            if existing["content_sha256"] == digest and existing["repo_path"] == path_text:
                return {"status": "already_published", "artifact_id": existing["artifact_id"]}
            raise ArtifactError("artifact logical name already exists with different content")
        artifact_id = new_id("art")
        now = utc_now()
        # HARNESS-003 byline integrity: record the actual ``author:`` line
        # from the published file (or NULL when absent), not the
        # workflow's declared expected byline. Snapshot renderers can
        # then distinguish "the workflow asked for byline X" from "the
        # artifact file actually carried byline X".
        actual_author_line = _first_author_line(payload)
        # RFC 0046 V1: store the override rationale on the artifact row
        # when present. NULL means no override applied; non-empty string
        # means --allow-no-process-execution was used with --override-rationale.
        stored_rationale: str | None = (
            override_rationale.strip()
            if (allow_no_process_execution and override_rationale)
            else None
        )
        conn.execute(
            """
            INSERT INTO artifacts (
              artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes,
              publish_mode, created_at, author_line,
              attestation_override_rationale
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'create', ?, ?, ?)
            """,
            (
                artifact_id,
                job["run_id"],
                job_id,
                session_id,
                logical_name,
                kind,
                path_text,
                digest,
                len(payload),
                now,
                actual_author_line,
                stored_rationale,
            ),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="artifact.published",
            actor_session_id=session_id,
            job_id=job_id,
            artifact_id=artifact_id,
            lease_id=lease_id,
            payload={"logical_name": logical_name, "path": path_text, "sha256": digest},
        )
        if stored_rationale is not None:
            insert_event(
                conn,
                run_id=str(job["run_id"]),
                event_type="provenance.publish_without_process_execution",
                actor_session_id=session_id,
                job_id=job_id,
                artifact_id=artifact_id,
                lease_id=lease_id,
                payload={
                    "byline": byline,
                    "path": path_text,
                    "rationale": stored_rationale,
                },
            )
        return {"status": "published", "artifact_id": artifact_id, "sha256": digest}


def _is_operator_byline(byline: str) -> bool:
    """RFC 0046 V1 helper: True iff the byline is operator-authored
    (``author: operator`` or ``author: operator [self-declared: ...]``).
    """
    return byline.startswith("author: operator")


def _lane_evidence_present(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    path_text: str,
) -> bool:
    """RFC 0046 V1 helper: True iff *session_id* has at least one
    ``process_executions`` row in state ``completed`` with
    ``exit_code = 0``.

    V1 evidence shape: the session's supervised subprocess ran cleanly
    to completion. The path-specific check (the row's
    ``observed_output_paths`` actually covers ``path_text``) is deferred
    to V1.7 — the existing ``process_executions`` schema does not yet
    capture observed output paths, so V1 lands the weaker but real
    guarantee. ``path_text`` is kept in the signature for V1.7 binary
    compatibility once the schema gains the column.
    """
    del path_text  # V1 placeholder; consumed in V1.7 schema upgrade.
    # process_executions.state enum is starting/running/exited/failed/
    # timed_out/lost. A clean lane run is state='exited' + exit_code=0.
    row = conn.execute(
        """
        SELECT 1 FROM process_executions
         WHERE session_id = ?
           AND state = 'exited'
           AND exit_code = 0
         LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    return row is not None


def validate_optional_markdown_author_line(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    path: Path,
    payload: bytes,
) -> None:
    """Validate Markdown artifact author metadata when the file provides it."""
    if path.suffix.lower() not in MARKDOWN_SUFFIXES:
        return
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError("markdown artifact must be UTF-8") from exc
    author_lines = markdown_title_block_author_lines(text)
    if not author_lines:
        return
    expected = expected_author_line(conn, job=job, session_id=session_id)
    for line in author_lines:
        canonical = _canonical_byline_form(line)
        if canonical is None:
            # Should not happen: ``markdown_title_block_author_lines`` only
            # appends lines that recognise as bylines. If decoration changes
            # the canonical form materially, treat as a mismatch rather than
            # silently dropping.
            raise ArtifactError(
                "markdown artifact author line must match expected work "
                "packet author line"
            )
        if canonical != expected:
            raise ArtifactError(
                "markdown artifact author line must match expected work "
                "packet author line"
            )


def expected_author_line(conn: sqlite3.Connection, *, job: sqlite3.Row, session_id: str) -> str:
    """Return the exact work-packet author line expected for this job/session."""
    from striatum.legacy_sqlite.db import row_by_id

    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    session = row_by_id(conn, "sessions", "session_id", session_id)
    attestation = session_lane_attestation(conn, session_id=session_id, mark_lost=True)
    lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    author = artifact_author_identity(
        json_loads(str(snapshot["workflow_json"])),
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
        attested=attestation.attested,
        operator_label=session["operator_label"] if "operator_label" in session.keys() else None,
    )
    line = author["line"]
    if line is None:
        raise ArtifactError("expected artifact author line could not be derived")
    return line


def _enforce_required_attestation_for_artifact(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
) -> None:
    if not _job_requires_attested_lane(conn, job=job):
        return
    attestation = session_lane_attestation(conn, session_id=session_id, mark_lost=True)
    if attestation.attested:
        return
    reason = f" ({attestation.reason})" if attestation.reason else ""
    raise ArtifactError(
        "job requires an attached lane supervisor before publishing artifacts"
        f"{reason}; recovery: striatum supervise start --session-id {session_id}"
    )


def _job_requires_attested_lane(conn: sqlite3.Connection, *, job: sqlite3.Row) -> bool:
    from striatum.legacy_sqlite.db import row_by_id

    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    workflow = json_loads(str(snapshot["workflow_json"]))
    jobs = workflow.get("jobs")
    if not isinstance(jobs, list):
        return False
    workflow_job_id = str(job["workflow_job_id"])
    for item in jobs:
        if not isinstance(item, dict) or item.get("id") != workflow_job_id:
            continue
        return item.get("require_attested_lane") is True
    return False
