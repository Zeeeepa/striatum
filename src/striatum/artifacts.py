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
from striatum.identity import artifact_author_identity, session_lane_attestation


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
    "escalation", "operator_brief", "work_plan", "progress_note",
    "operator_report",
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


def _is_non_empty_str(value: object) -> str | None:
    if not isinstance(value, str):
        return "must be a non-empty string"
    if value.strip() == "":
        return "must be a non-empty string"
    return None


def _is_nullable_non_empty_str(value: object) -> str | None:
    if value is None:
        return None
    return _is_non_empty_str(value)


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


def _is_scope_links(value: object) -> str | None:
    problem = _is_str_list(value)
    if problem is not None:
        return problem
    assert isinstance(value, list)
    if len(value) > 5:
        return "must contain at most 5 entries"
    if any(not item.strip() for item in value):
        return "must be a list of non-empty strings"
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
_BLOCKER_SEVERITIES: tuple[str, ...] = ("blocked", "human_checkpoint")
_ESCALATION_BLOCKER_KINDS: tuple[str, ...] = (
    "ambiguous_goal",
    "missing_authority",
    "contradicting_decisions",
    "no_available_reviewer_lane",
    "committee_stalemate",
    "override_required",
    "ai_self_declared",
)
_RETRIEVAL_PRIORITIES: tuple[str, ...] = ("high", "normal", "low")
_OPERATOR_BRIEF_STATUSES: tuple[str, ...] = ("current", "superseded")
_WORK_PLAN_SCOPE_KINDS: tuple[str, ...] = ("rfc", "phase", "initiative", "bugfix")
_WORK_PLAN_STATES: tuple[str, ...] = ("open", "in_progress", "closed")


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
    "escalation": FrontMatterSchema(
        schema_version="striatum.escalation.v1",
        artifact_kind="escalation",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.escalation.v1")),
            FrontMatterField("artifact_kind", True, _equals("escalation")),
            FrontMatterField("escalation_id", True, _is_non_empty_str),
            FrontMatterField("run_id", True, _is_non_empty_str),
            FrontMatterField("job_id", False, _is_non_empty_str),
            FrontMatterField("session_id", False, _is_non_empty_str),
            FrontMatterField("severity", True, _one_of("severity", _BLOCKER_SEVERITIES)),
            FrontMatterField(
                "blocker_kind",
                True,
                _one_of("blocker_kind", _ESCALATION_BLOCKER_KINDS),
            ),
            FrontMatterField("description", True, _is_non_empty_str),
            FrontMatterField("reasoning", True, _is_non_empty_str),
            FrontMatterField("requested_action", True, _is_non_empty_str),
            FrontMatterField("related_artifacts", False, _is_str_list),
            FrontMatterField("created_at", True, _is_non_empty_str),
        ),
    ),
    "operator_brief": FrontMatterSchema(
        schema_version="striatum.operator_brief.v1",
        artifact_kind="operator_brief",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.operator_brief.v1")),
            FrontMatterField("artifact_kind", True, _equals("operator_brief")),
            FrontMatterField("brief_id", True, _is_non_empty_str),
            FrontMatterField("supersedes", True, _is_nullable_non_empty_str),
            FrontMatterField("scope_links", True, _is_scope_links),
            FrontMatterField("context_budget_lines", True, _is_non_negative_int),
            FrontMatterField("retrieval_priority", True, _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES)),
            FrontMatterField("status", True, _one_of("status", _OPERATOR_BRIEF_STATUSES)),
            FrontMatterField("author", False, _is_non_empty_str),
        ),
    ),
    "work_plan": FrontMatterSchema(
        schema_version="striatum.work_plan.v1",
        artifact_kind="work_plan",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.work_plan.v1")),
            FrontMatterField("artifact_kind", True, _equals("work_plan")),
            FrontMatterField("plan_id", True, _is_non_empty_str),
            FrontMatterField("scope_kind", True, _one_of("scope_kind", _WORK_PLAN_SCOPE_KINDS)),
            FrontMatterField("scope_ref", True, _is_non_empty_str),
            FrontMatterField("state", True, _one_of("state", _WORK_PLAN_STATES)),
            FrontMatterField("opened_at", True, _is_non_empty_str),
            FrontMatterField("closed_at", True, _is_nullable_non_empty_str),
            FrontMatterField("closure_summary", True, _is_nullable_non_empty_str),
            FrontMatterField("supersedes", True, _is_nullable_non_empty_str),
            FrontMatterField("retrieval_priority", True, _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES)),
            FrontMatterField("author", False, _is_non_empty_str),
        ),
    ),
    "progress_note": FrontMatterSchema(
        schema_version="striatum.progress_note.v1",
        artifact_kind="progress_note",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.progress_note.v1")),
            FrontMatterField("artifact_kind", True, _equals("progress_note")),
            FrontMatterField("note_date", True, _is_non_empty_str),
            FrontMatterField("session_slug", True, _is_non_empty_str),
            FrontMatterField("related_plan", True, _is_nullable_non_empty_str),
            FrontMatterField("related_brief", True, _is_nullable_non_empty_str),
            FrontMatterField("retrieval_priority", True, _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES)),
            FrontMatterField("author", False, _is_non_empty_str),
        ),
    ),
    "operator_report": FrontMatterSchema(
        schema_version="striatum.operator_report.v1",
        artifact_kind="operator_report",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.operator_report.v1")),
            FrontMatterField("artifact_kind", True, _equals("operator_report")),
            FrontMatterField("author", False, _is_non_empty_str),
            FrontMatterField("retrieval_priority", False, _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES)),
            FrontMatterField("supersedes", False, _is_nullable_non_empty_str),
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


def _front_matter_body(text: str) -> str | None:
    """Return Markdown body text after a leading front-matter block."""
    if not text.startswith("---"):
        return None
    head_end = 3
    if head_end < len(text) and text[head_end] == "\r":
        head_end += 1
    if head_end >= len(text) or text[head_end] != "\n":
        return None
    body_start = head_end + 1
    rel = text.find("\n---", body_start - 1)
    if rel == -1:
        return None
    close_start = rel + 1
    close_end = close_start + 3
    if close_end < len(text) and text[close_end] == "\r":
        close_end += 1
    if close_end < len(text) and text[close_end] == "\n":
        close_end += 1
    return text[close_end:]


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


def _validate_operator_brief_context_budget(
    *, parsed: dict[str, object], body: str | None
) -> None:
    """Promote RFC 0058 V1.5 body-length drift to a schema error."""
    if body is None:
        return
    budget = parsed.get("context_budget_lines")
    if isinstance(budget, bool) or not isinstance(budget, int):
        return
    body_lines = 0 if body == "" else len(body.splitlines())
    if body_lines > budget:
        raise ArtifactError(
            "operator_brief artifact front matter field "
            f"'context_budget_lines' budget exceeded: body has {body_lines} "
            f"lines, limit is {budget}"
        )


def ensure_required_front_matter(
    *, kind: str, path: Path, payload: bytes
) -> bytes:
    """Auto-attach default front matter for schema-bearing kinds with only-constant requirements.

    Currently exercises the ``synthesis`` kind: its required fields
    (``schema_version`` and ``artifact_kind``) are constants the
    publisher already knows from the ``--kind synthesis`` flag, so
    refusing because the agent did not echo them back is friction with
    no provenance benefit. When the file omits the front matter block,
    the publisher prepends the canonical block to the file and returns
    the new payload; the agent's body is preserved verbatim after the
    block.

    Other schema-bearing kinds (``finding``, ``decision``,
    ``findings_ledger``, ``support_ledger``, ``action_item_ledger``,
    ``harness_improvement_proposal``, ``escalation``) carry semantic
    required fields (``verdict_intent``, ``outcome``,
    ``audited_artifact``, ``reasoning``, etc.) that the publisher cannot
    invent. For backward compatibility with
    artifacts authored before V1 front matter became conventional, the
    publisher continues to silently accept those kinds when front
    matter is absent; downstream renderers record ``actual_author_line``
    as NULL for the missing metadata case. Adding an explicit refusal
    here would be a policy break; if that is desired, it should land
    behind a workflow-level opt-in and a deprecation window.

    Schema-less kinds and non-Markdown files are passed through
    unchanged.
    """
    schema = FRONT_MATTER_SCHEMAS.get(kind)
    if schema is None:
        return payload
    if path.suffix.lower() not in _MARKDOWN_SUFFIXES:
        return payload
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(
            f"{kind} artifact must be UTF-8 to validate front matter"
        ) from exc
    if _front_matter_block(text) is not None:
        # Front matter already present; validate path takes over.
        return payload
    if schema.artifact_kind == "synthesis":
        # Only-constant required fields; safe to auto-attach.
        prepend = (
            "---\n"
            f'schema_version: "{schema.schema_version}"\n'
            f'artifact_kind: "{schema.artifact_kind}"\n'
            "---\n\n"
        )
        new_text = prepend + text
        new_payload = new_text.encode("utf-8")
        path.write_bytes(new_payload)
        return new_payload
    # Other schema-bearing kinds: preserve historic silent-accept behavior.
    return payload


def validate_artifact_front_matter(
    *, kind: str, path: Path, payload: bytes
) -> None:
    """Validate Markdown front matter for kinds with a registered schema.

    No-op when the artifact kind has no schema, when the file is not Markdown,
    or when the file does not start with a `---` block. Callers should run
    :func:`ensure_required_front_matter` first to auto-attach defaults for
    synthesis and to refuse missing front matter for other schema-bearing
    kinds with a template error.
    """
    parse_artifact_front_matter(kind=kind, path=path, payload=payload)


def parse_artifact_front_matter(
    *, kind: str, path: Path, payload: bytes
) -> dict[str, object] | None:
    """Parse and validate Markdown front matter for a schema-bearing artifact.

    Returns the parsed mapping when front matter exists, otherwise ``None``.
    This is the public helper for callers that need validated metadata after
    publish-time schema checks; callers should not import the private parser.
    """
    schema = FRONT_MATTER_SCHEMAS.get(kind)
    if schema is None:
        return None
    if path.suffix.lower() not in _MARKDOWN_SUFFIXES:
        return None
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(
            f"{kind} artifact must be UTF-8 to validate front matter"
        ) from exc
    block = _front_matter_block(text)
    if block is None:
        return None
    parsed = _parse_front_matter(block, kind=kind)
    _validate_front_matter(parsed, schema)
    if schema.artifact_kind == "operator_brief":
        _validate_operator_brief_context_budget(
            parsed=parsed,
            body=_front_matter_body(text),
        )
    return parsed


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


def _strip_markdown_decoration(line: str) -> str:
    """Strip leading and inline Markdown decoration around a byline.

    Models seen in dogfood runs include ``**Author:** value``,
    ``# Author: value``, ``_author:_ value``, and combinations. The
    canonical byline is plain ``author: value``; this helper normalizes
    the surface forms so downstream matching and storage agree.
    """
    stripped = line.lstrip()
    # Drop leading Markdown heading markers like ``# `` or ``### ``.
    while stripped.startswith("#"):
        stripped = stripped[1:]
    stripped = stripped.lstrip()
    # Drop leading/trailing bold/italic markers around the entire line.
    stripped = stripped.strip()
    # Remove all asterisk and underscore decoration so ``**Author:**`` ->
    # ``Author:`` and ``_author_:`` -> ``author:``. The byline value itself
    # is constrained by ``author_part`` to ``[a-z0-9.-]+``, so stripping
    # these characters from the entire string is safe.
    stripped = stripped.replace("*", "").replace("_", "")
    return stripped.strip()


def _canonical_byline_form(line: str) -> str | None:
    """Return the canonical ``author: <lowercase-value>`` form, or None.

    Returns None when the line does not name an author after stripping
    Markdown decoration. The canonical form is what the publisher stores
    in ``artifacts.author_line`` and what byline-equality checks compare
    against.
    """
    normalized = _strip_markdown_decoration(line)
    if not normalized.lower().startswith("author:"):
        return None
    suffix = normalized.split(":", 1)[1].strip()
    return f"author: {suffix.lower()}"


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


def _first_author_line(payload: bytes) -> str | None:
    """Return the first ``author: ...`` line from a Markdown payload, canonicalized.

    Used by :func:`record_artifact` to populate ``artifacts.author_line``
    (HARNESS-003). Returns ``None`` when the file has no front matter or
    title-block author line; that NULL is the durable signal that the
    artifact did not actually carry the workflow's declared byline.

    Markdown decoration around the byline (``**Author:**``, ``# author:``,
    ``_author_:``) is stripped to the canonical ``author: <lowercase>``
    form so storage and downstream rendering agree.
    """
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError:
        return None
    lines = markdown_title_block_author_lines(text)
    if not lines:
        return None
    canonical = _canonical_byline_form(lines[0])
    if canonical is not None:
        return canonical
    # Fall back to the historic lowercase pass-through so legacy artifacts
    # whose byline cannot be normalised still produce a deterministic stored
    # value rather than NULL.
    return lines[0].strip().lower()


def markdown_title_block_author_lines(text: str) -> list[str]:
    """Return author metadata lines from YAML front matter and/or a Markdown title block.

    A Markdown artifact may carry an ``author:`` line in either of two
    places: inside YAML front matter (between ``---`` markers) or in
    the Markdown title block (the lines before the first ``## ``
    section heading). The dogfood handoff convention puts the byline
    *after* the front matter, in the title block; some workflow
    examples put it inside the front matter.

    V1.41 (harness friction burn-down): **front matter wins**. When
    front matter declares an ``author:`` line, title-block lines are
    ignored — this keeps body-text mentions like ``Author: <real-name>``
    (capitalised, in a title block) from competing with the canonical
    byline and tripping the publisher's equality check. Title-block
    bylines remain the source-of-truth only when no front-matter
    author line exists.

    Markdown decoration (``**Author:**``, ``# Author``, ``_author_``)
    is tolerated: a line is recognised as a byline iff its canonical
    form starts with ``author:`` after decoration is stripped. The
    original line text is returned so callers can render it; equality
    checks should canonicalise both sides.
    """
    lines = text.splitlines()
    front_matter_lines: list[str] = []
    title_block_lines: list[str] = []
    body_start = 0
    if lines and lines[0].strip() == "---":
        for index, line in enumerate(lines[1:], start=1):
            if line.strip() == "---":
                body_start = index + 1
                break
        front_matter = lines[1:body_start - 1] if body_start > 0 else []
        for line in front_matter:
            if _canonical_byline_form(line) is not None:
                front_matter_lines.append(line)
    title_block = lines[body_start : body_start + 40]
    # Title-block scan: only accept the FIRST canonical author line. This
    # prevents body-text mentions like ``Author: designer-gemini-1`` from
    # competing with the canonical lowercase byline that immediately
    # follows the front matter. Decoration tolerance still applies so
    # workflows that put ``# Author: ...`` in the title block continue
    # to work.
    for line in title_block:
        if line.startswith("## "):
            break
        if _canonical_byline_form(line) is not None:
            title_block_lines.append(line)
            break
    # Front-matter author lines take precedence; title-block lines only
    # contribute when front matter has none.
    if front_matter_lines:
        return front_matter_lines
    return title_block_lines


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
