"""Neutral artifact contract helpers.

This module is intentionally free of legacy repo-local state access. It may be
imported by daemon PostgreSQL handlers that only need artifact-kind constants,
front-matter validation, or Markdown byline parsing.
"""

from __future__ import annotations

import json
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from striatum.errors import ArtifactError

MARKDOWN_SUFFIXES = {".md", ".markdown"}
_MARKDOWN_SUFFIXES: tuple[str, ...] = (".md", ".markdown")


# Artifact-kind validation lives in Python (migration v5 dropped the SQL CHECK on
# `artifacts.artifact_kind`). Adding a new kind means extending this set and,
# optionally, registering a `FrontMatterSchema` below.
ALLOWED_ARTIFACT_KINDS: frozenset[str] = frozenset(
    {
        "prompt",
        "finding",
        "findings_ledger",
        "synthesis",
        "marker",
        "handoff",
        "decision",
        "patch_summary",
        "test_report",
        "other",
        "support_ledger",
        "action_item_ledger",
        "harness_improvement_proposal",
        "escalation",
        "operator_brief",
        "work_plan",
        "progress_note",
        "operator_report",
        "commit_request",
        "pr_request",
    }
)


@dataclass(frozen=True)
class FrontMatterField:
    """One declarative front-matter field rule."""

    name: str
    required: bool
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


def _is_non_empty_str_list(value: object) -> str | None:
    problem = _is_str_list(value)
    if problem is not None:
        return problem
    assert isinstance(value, list)
    if not value:
        return "must be a non-empty list of non-empty strings"
    if any(not item.strip() for item in value):
        return "must be a non-empty list of non-empty strings"
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
_GIT_CONFIRMATION_STATUSES: tuple[str, ...] = (
    "pending",
    "operator_confirmed",
    "human_confirmed",
    "refused",
)
_PR_CONFIRMATION_STATUSES: tuple[str, ...] = ("pending", "human_confirmed", "refused")


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
                "verdict_intent",
                True,
                _one_of("verdict_intent", _FINDING_VERDICT_INTENTS),
            ),
            FrontMatterField("severity", False, _one_of("severity", _FINDING_SEVERITIES)),
            FrontMatterField("tags", False, _is_str_list),
        ),
    ),
    "findings_ledger": FrontMatterSchema(
        schema_version="striatum.findings_ledger.v1",
        artifact_kind="findings_ledger",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.findings_ledger.v1")),
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
            FrontMatterField("schema_version", True, _equals("striatum.support_ledger.v1")),
            FrontMatterField("artifact_kind", True, _equals("support_ledger")),
            FrontMatterField("audited_artifact", True, _is_str),
            FrontMatterField("claim_count", False, _is_non_negative_int),
        ),
    ),
    "action_item_ledger": FrontMatterSchema(
        schema_version="striatum.action_item_ledger.v1",
        artifact_kind="action_item_ledger",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.action_item_ledger.v1")),
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
            FrontMatterField("artifact_kind", True, _equals("harness_improvement_proposal")),
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
            FrontMatterField(
                "retrieval_priority",
                True,
                _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES),
            ),
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
            FrontMatterField(
                "retrieval_priority",
                True,
                _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES),
            ),
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
            FrontMatterField(
                "retrieval_priority",
                True,
                _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES),
            ),
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
            FrontMatterField(
                "retrieval_priority",
                False,
                _one_of("retrieval_priority", _RETRIEVAL_PRIORITIES),
            ),
            FrontMatterField("supersedes", False, _is_nullable_non_empty_str),
        ),
    ),
    "commit_request": FrontMatterSchema(
        schema_version="striatum.commit_request.v1",
        artifact_kind="commit_request",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.commit_request.v1")),
            FrontMatterField("artifact_kind", True, _equals("commit_request")),
            FrontMatterField("request_id", True, _is_non_empty_str),
            FrontMatterField("run_id", False, _is_non_empty_str),
            FrontMatterField("base_head", True, _is_non_empty_str),
            FrontMatterField("branch", True, _is_non_empty_str),
            FrontMatterField("git_snapshot_hash", True, _is_non_empty_str),
            FrontMatterField("included_paths", True, _is_non_empty_str_list),
            FrontMatterField("reviewed_artifacts", False, _is_non_empty_str_list),
            FrontMatterField("commit_message", True, _is_non_empty_str),
            FrontMatterField("rationale", True, _is_non_empty_str),
            FrontMatterField(
                "confirmation_status",
                True,
                _one_of("confirmation_status", _GIT_CONFIRMATION_STATUSES),
            ),
            FrontMatterField("confirmed_by", False, _is_nullable_non_empty_str),
            FrontMatterField("confirmed_at", False, _is_nullable_non_empty_str),
        ),
    ),
    "pr_request": FrontMatterSchema(
        schema_version="striatum.pr_request.v1",
        artifact_kind="pr_request",
        fields=(
            FrontMatterField("schema_version", True, _equals("striatum.pr_request.v1")),
            FrontMatterField("artifact_kind", True, _equals("pr_request")),
            FrontMatterField("request_id", True, _is_non_empty_str),
            FrontMatterField("run_id", False, _is_non_empty_str),
            FrontMatterField("target_branch", True, _is_non_empty_str),
            FrontMatterField("summary", True, _is_non_empty_str),
            FrontMatterField("body_draft", True, _is_non_empty_str),
            FrontMatterField("related_commit_request", False, _is_nullable_non_empty_str),
            FrontMatterField("local_commit_sha", False, _is_nullable_non_empty_str),
            FrontMatterField("provider_target", False, _is_nullable_non_empty_str),
            FrontMatterField(
                "confirmation_status",
                True,
                _one_of("confirmation_status", _PR_CONFIRMATION_STATUSES),
            ),
            FrontMatterField("confirmed_by", False, _is_nullable_non_empty_str),
            FrontMatterField("confirmed_at", False, _is_nullable_non_empty_str),
        ),
    ),
}


def _front_matter_block(text: str) -> str | None:
    """Extract the `---`-delimited front-matter block, if any."""
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
        raise ArtifactError(
            "artifact front matter block is not closed; "
            "add a `---` terminator after the metadata"
        )
    block = text[body_start : rel + 1]
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
    """Parse a minimal `key: <json-value>` front-matter block."""
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
        value_text = line[sep + 1 :].strip()
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


def _validate_pr_request_source(parsed: dict[str, object]) -> None:
    """Require a PR request to cite a commit request or local commit."""
    related_commit_request = parsed.get("related_commit_request")
    local_commit_sha = parsed.get("local_commit_sha")
    if related_commit_request is None and local_commit_sha is None:
        raise ArtifactError(
            "pr_request artifact front matter requires at least one of "
            "'related_commit_request' or 'local_commit_sha'"
        )


def ensure_required_front_matter(*, kind: str, path: Path, payload: bytes) -> bytes:
    """Auto-attach default front matter for schema-bearing kinds when safe."""
    schema = FRONT_MATTER_SCHEMAS.get(kind)
    if schema is None:
        return payload
    if path.suffix.lower() not in _MARKDOWN_SUFFIXES:
        return payload
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(f"{kind} artifact must be UTF-8 to validate front matter") from exc
    if _front_matter_block(text) is not None:
        return payload
    if schema.artifact_kind == "synthesis":
        prepend = (
            "---\n"
            f'schema_version: "{schema.schema_version}"\n'
            f'artifact_kind: "{schema.artifact_kind}"\n'
            "---\n\n"
        )
        new_payload = (prepend + text).encode("utf-8")
        path.write_bytes(new_payload)
        return new_payload
    return payload


def validate_artifact_front_matter(*, kind: str, path: Path, payload: bytes) -> None:
    """Validate Markdown front matter for kinds with a registered schema."""
    parse_artifact_front_matter(kind=kind, path=path, payload=payload)


def parse_artifact_front_matter(
    *, kind: str, path: Path, payload: bytes
) -> dict[str, object] | None:
    """Parse and validate Markdown front matter for a schema-bearing artifact."""
    schema = FRONT_MATTER_SCHEMAS.get(kind)
    if schema is None:
        return None
    if path.suffix.lower() not in _MARKDOWN_SUFFIXES:
        return None
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(f"{kind} artifact must be UTF-8 to validate front matter") from exc
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
    if schema.artifact_kind == "pr_request":
        _validate_pr_request_source(parsed)
    return parsed


def _strip_markdown_decoration(line: str) -> str:
    """Strip leading and inline Markdown decoration around a byline."""
    stripped = line.lstrip()
    while stripped.startswith("#"):
        stripped = stripped[1:]
    stripped = stripped.lstrip().strip()
    stripped = stripped.replace("*", "").replace("_", "")
    return stripped.strip()


def _canonical_byline_form(line: str) -> str | None:
    """Return the canonical ``author: <lowercase-value>`` form, or None."""
    normalized = _strip_markdown_decoration(line)
    if not normalized.lower().startswith("author:"):
        return None
    suffix = normalized.split(":", 1)[1].strip()
    return f"author: {suffix.lower()}"


def markdown_title_block_author_lines(text: str) -> list[str]:
    """Return author metadata lines from front matter or a Markdown title block."""
    lines = text.splitlines()
    front_matter_lines: list[str] = []
    title_block_lines: list[str] = []
    body_start = 0
    if lines and lines[0].strip() == "---":
        for index, line in enumerate(lines[1:], start=1):
            if line.strip() == "---":
                body_start = index + 1
                break
        front_matter = lines[1 : body_start - 1] if body_start > 0 else []
        for line in front_matter:
            if _canonical_byline_form(line) is not None:
                front_matter_lines.append(line)
    title_block = lines[body_start : body_start + 40]
    for line in title_block:
        if line.startswith("## "):
            break
        if _canonical_byline_form(line) is not None:
            title_block_lines.append(line)
            break
    if front_matter_lines:
        return front_matter_lines
    return title_block_lines


def _first_author_line(payload: bytes) -> str | None:
    """Return the first ``author: ...`` line from a Markdown payload, canonicalized."""
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
    return lines[0].strip().lower()


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
    "markdown_title_block_author_lines",
    "parse_artifact_front_matter",
    "validate_artifact_front_matter",
]
