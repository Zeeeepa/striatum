"""Artifact publisher for durable repo outputs."""

from __future__ import annotations

import json
import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

from striatum.db import (
    active_lease_for,
    active_worktree_for_job,
    insert_event,
    json_loads,
    new_id,
    path_allowed,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    transaction,
    utc_now,
)
from striatum.errors import ArtifactError
from striatum.identity import artifact_author_identity


MARKDOWN_SUFFIXES = {".md", ".markdown"}


# --- Allowed artifact kinds ---------------------------------------------------------
#
# Artifact-kind validation lives in Python (migration v5 dropped the SQL CHECK on
# `artifacts.artifact_kind`). Adding a new kind now means extending this set and,
# optionally, registering a `FrontMatterSchema` below.

ALLOWED_ARTIFACT_KINDS: frozenset[str] = frozenset({
    "prompt", "finding", "findings_ledger", "synthesis", "marker",
    "handoff", "decision", "patch_summary", "test_report", "other",
    "support_ledger", "action_item_ledger", "harness_improvement_proposal",
})


# --- Front-matter schema definitions -------------------------------------------------
#
# Durable Markdown artifacts MAY include YAML-style `---` front matter. When a
# kind has a registered schema and the file starts with a `---` block, the
# publisher validates the parsed front matter against the schema. Files without
# front matter are accepted unchanged (preserves the prior loose behavior).
#
# The publisher records artifacts and never mutates files. Front-matter
# validation is read-only.


@dataclass(frozen=True)
class FrontMatterField:
    """One declarative front-matter field rule."""

    name: str
    required: bool
    # Predicate returning a description of the rule when the value is invalid,
    # or None when the value is acceptable. The predicate is only invoked when
    # the field is present.
    validator: Callable[[object], str | None]


@dataclass(frozen=True)
class FrontMatterSchema:
    """A per-artifact-kind front-matter rule set."""

    schema_version: str
    artifact_kind: str
    fields: tuple[FrontMatterField, ...]


def _is_str(value: object) -> str | None:
    return None if isinstance(value, str) else "must be a string"


def _is_bool(value: object) -> str | None:
    return None if isinstance(value, bool) else "must be a boolean"


def _is_non_negative_int(value: object) -> str | None:
    if isinstance(value, bool) or not isinstance(value, int):
        return "must be a non-negative integer"
    return None if value >= 0 else "must be a non-negative integer"


def _is_str_list(value: object) -> str | None:
    if not isinstance(value, list):
        return "must be a list of strings"
    for item in value:
        if not isinstance(item, str):
            return "must be a list of strings"
    return None


def _one_of(name: str, choices: tuple[str, ...]) -> Callable[[object], str | None]:
    expected = "|".join(choices)

    def check(value: object) -> str | None:
        if not isinstance(value, str):
            return f"must be one of {expected}"
        if value not in choices:
            return f"must be one of {expected}, got {value!r}"
        return None

    check.__name__ = f"_one_of_{name}"
    return check


def _equals(expected: str) -> Callable[[object], str | None]:
    def check(value: object) -> str | None:
        if value == expected:
            return None
        return f"must equal {expected!r}, got {value!r}"

    check.__name__ = f"_equals_{expected}"
    return check


_DECISION_OUTCOMES: tuple[str, ...] = ("accepted", "rejected", "accepted_with_follow_up")
_FINDING_VERDICT_INTENTS: tuple[str, ...] = (
    "accept",
    "accept_with_findings",
    "needs_revision",
    "reject",
)
_FINDING_SEVERITIES: tuple[str, ...] = ("info", "low", "medium", "high", "critical")
_HARNESS_PROPOSAL_TARGETS: tuple[str, ...] = (
    "prompt",
    "workflow",
    "spec",
    "defaults",
    "documentation",
)


FRONT_MATTER_SCHEMAS: dict[str, FrontMatterSchema] = {
    "decision": FrontMatterSchema(
        schema_version="striatum.decision.v1",
        artifact_kind="decision",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.decision.v1")),
            FrontMatterField("artifact_kind", True, _equals("decision")),
            FrontMatterField("decision_id", True, _is_str),
            FrontMatterField("run_id", True, _is_str),
            FrontMatterField("owner", True, _equals("human")),
            FrontMatterField("outcome", True, _one_of("outcome", _DECISION_OUTCOMES)),
            FrontMatterField("follow_up_required", True, _is_bool),
            FrontMatterField("title", True, _is_str),
            FrontMatterField("created_at", True, _is_str),
        ),
    ),
    "finding": FrontMatterSchema(
        schema_version="striatum.finding.v1",
        artifact_kind="finding",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.finding.v1")),
            FrontMatterField("artifact_kind", True, _equals("finding")),
            FrontMatterField(
                "verdict_intent", True, _one_of("verdict_intent", _FINDING_VERDICT_INTENTS)
            ),
            FrontMatterField("severity", False, _one_of("severity", _FINDING_SEVERITIES)),
            FrontMatterField("tags", False, _is_str_list),
        ),
    ),
    "findings_ledger": FrontMatterSchema(
        schema_version="striatum.findings_ledger.v1",
        artifact_kind="findings_ledger",
        fields=(
            FrontMatterField(
                "schema_version", True, _equals("striatum.findings_ledger.v1")
            ),
            FrontMatterField("artifact_kind", True, _equals("findings_ledger")),
            FrontMatterField("summary_count", True, _is_non_negative_int),
            FrontMatterField("entries_path", False, _is_str),
        ),
    ),
    "synthesis": FrontMatterSchema(
        schema_version="striatum.synthesis.v1",
        artifact_kind="synthesis",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.synthesis.v1")),
            FrontMatterField("artifact_kind", True, _equals("synthesis")),
            FrontMatterField("inputs", False, _is_str_list),
        ),
    ),
    "support_ledger": FrontMatterSchema(
        schema_version="striatum.support_ledger.v1",
        artifact_kind="support_ledger",
        fields=(
            FrontMatterField(
                "schema_version", True, _equals("striatum.support_ledger.v1")
            ),
            FrontMatterField("artifact_kind", True, _equals("support_ledger")),
            FrontMatterField("audited_artifact", True, _is_str),
            FrontMatterField("claim_count", False, _is_non_negative_int),
        ),
    ),
    "action_item_ledger": FrontMatterSchema(
        schema_version="striatum.action_item_ledger.v1",
        artifact_kind="action_item_ledger",
        fields=(
            FrontMatterField(
                "schema_version", True, _equals("striatum.action_item_ledger.v1")
            ),
            FrontMatterField("artifact_kind", True, _equals("action_item_ledger")),
            FrontMatterField("source_review_artifact", True, _is_str),
            FrontMatterField("revision_round", True, _is_non_negative_int),
            FrontMatterField("total_items", False, _is_non_negative_int),
        ),
    ),
    "harness_improvement_proposal": FrontMatterSchema(
        schema_version="striatum.harness_improvement_proposal.v1",
        artifact_kind="harness_improvement_proposal",
        fields=(
            FrontMatterField(
                "schema_version",
                True,
                _equals("striatum.harness_improvement_proposal.v1"),
            ),
            FrontMatterField(
                "artifact_kind", True, _equals("harness_improvement_proposal")
            ),
            FrontMatterField("target", True, _one_of("target", _HARNESS_PROPOSAL_TARGETS)),
            FrontMatterField("expected_benefit", True, _is_str),
            FrontMatterField("risk", False, _is_str),
            FrontMatterField("rollback", False, _is_str),
        ),
    ),
}


_MARKDOWN_SUFFIXES: tuple[str, ...] = (".md", ".markdown")


def _front_matter_block(text: str) -> str | None:
    """Extract the `---`-delimited front-matter block, if any.

    Returns the inner block text without the delimiter lines, or None when no
    front matter is present.
    """
    # The block must start at the very first line. Allow either "\r\n" or "\n".
    if not text.startswith("---"):
        return None
    # Require the opening "---" to be followed by an end-of-line.
    head_end = 3
    if head_end < len(text) and text[head_end] == "\r":
        head_end += 1
    if head_end >= len(text) or text[head_end] != "\n":
        return None
    body_start = head_end + 1
    # Find a closing "---" at the start of a line.
    needle = "\n---"
    rel = text.find(needle, body_start - 1)
    if rel == -1:
        raise ArtifactError(
            "artifact front matter block is not closed; "
            "add a `---` terminator after the metadata"
        )
    block = text[body_start:rel + 1]
    # Strip the trailing newline before the closing fence to keep parsing simple.
    if block.endswith("\n"):
        block = block[:-1]
    return block


def _parse_front_matter(block: str, *, kind: str) -> dict[str, object]:
    """Parse a minimal `key: <json-value>` block.

    Each non-empty, non-comment line must be ``key: <json-value>``. Values are
    decoded with the standard JSON parser, so quoted strings, booleans, ints,
    floats, ``null``, and JSON-style lists/objects all work. Bare scalars are
    not accepted; the user must JSON-quote string values. This keeps parsing
    deterministic without adding a YAML dependency.
    """
    parsed: dict[str, object] = {}
    for line_number, raw_line in enumerate(block.splitlines(), start=1):
        line = raw_line.rstrip("\r")
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if line.startswith((" ", "\t")):
            raise ArtifactError(
                f"{kind} artifact front matter line {line_number} is indented; "
                "nested values are not supported, write each key at column 0"
            )
        sep = line.find(":")
        if sep == -1:
            raise ArtifactError(
                f"{kind} artifact front matter line {line_number} is not 'key: value'"
            )
        key = line[:sep].strip()
        if not key:
            raise ArtifactError(
                f"{kind} artifact front matter line {line_number} has an empty key"
            )
        value_text = line[sep + 1:].strip()
        if not value_text:
            raise ArtifactError(
                f"{kind} artifact front matter field {key!r} has no value"
            )
        try:
            value = json.loads(value_text)
        except json.JSONDecodeError as exc:
            raise ArtifactError(
                f"{kind} artifact front matter field {key!r} value must be JSON-encoded "
                f"(quote strings, use true/false/null, JSON lists); got {value_text!r}"
            ) from exc
        if key in parsed:
            raise ArtifactError(
                f"{kind} artifact front matter field {key!r} is declared more than once"
            )
        parsed[key] = value
    return parsed


def _validate_front_matter(parsed: dict[str, object], schema: FrontMatterSchema) -> None:
    """Apply schema rules to a parsed front-matter mapping."""
    declared = {field.name for field in schema.fields}
    for field in schema.fields:
        if field.name not in parsed:
            if field.required:
                raise ArtifactError(
                    f"{schema.artifact_kind} artifact front matter "
                    f"missing required field {field.name!r}"
                )
            continue
        problem = field.validator(parsed[field.name])
        if problem is not None:
            raise ArtifactError(
                f"{schema.artifact_kind} artifact front matter "
                f"field {field.name!r} {problem}"
            )
    extra = sorted(set(parsed) - declared)
    if extra:
        raise ArtifactError(
            f"{schema.artifact_kind} artifact front matter has unknown fields: "
            f"{', '.join(extra)}"
        )


def validate_artifact_front_matter(
    *, kind: str, path: Path, payload: bytes
) -> None:
    """Validate optional Markdown front matter for kinds with a registered schema.

    No-op when the artifact kind has no schema, when the file is not Markdown,
    or when the file does not start with a `---` block.
    """
    schema = FRONT_MATTER_SCHEMAS.get(kind)
    if schema is None:
        return
    if path.suffix.lower() not in _MARKDOWN_SUFFIXES:
        return
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(
            f"{kind} artifact must be UTF-8 to validate front matter"
        ) from exc
    block = _front_matter_block(text)
    if block is None:
        return
    parsed = _parse_front_matter(block, kind=kind)
    _validate_front_matter(parsed, schema)


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
) -> dict[str, object]:
    """Record an artifact reference after validating write scope."""
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
        validate_optional_markdown_author_line(
            conn,
            job=job,
            session_id=session_id,
            path=path,
            payload=payload,
        )
        validate_artifact_front_matter(kind=kind, path=path, payload=payload)
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
        conn.execute(
            """
            INSERT INTO artifacts (
              artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes,
              publish_mode, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'create', ?)
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
        return {"status": "published", "artifact_id": artifact_id, "sha256": digest}


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
        if line.strip() != expected:
            raise ArtifactError("markdown artifact author line must match expected work packet author line")


def markdown_title_block_author_lines(text: str) -> list[str]:
    """Return author metadata lines from YAML front matter or a Markdown title block."""
    lines = text.splitlines()
    if lines and lines[0].strip() == "---":
        front_matter: list[str] = []
        for line in lines[1:]:
            if line.strip() == "---":
                break
            front_matter.append(line)
        return [line for line in front_matter if line.strip().lower().startswith("author:")]

    title_block = lines[:40]
    author_lines: list[str] = []
    for line in title_block:
        if line.startswith("## "):
            break
        stripped = line.strip()
        if stripped.lower().startswith("author:"):
            author_lines.append(line)
    return author_lines


def expected_author_line(conn: sqlite3.Connection, *, job: sqlite3.Row, session_id: str) -> str:
    """Return the exact work-packet author line expected for this job/session."""
    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    session = row_by_id(conn, "sessions", "session_id", session_id)
    lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    author = artifact_author_identity(
        json_loads(str(snapshot["workflow_json"])),
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
    )
    line = author["line"]
    if line is None:
        raise ArtifactError("expected artifact author line could not be derived")
    return line
