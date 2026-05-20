"""Closed-source corpus row enumeration for RFC 0044."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any, Iterable

from striatum.corpus import git as git_helpers
from striatum.corpus.redaction import (
    redact_commit_message,
    validate_source_path,
)
from striatum.corpus.types import CorpusProvenance, CorpusRow

ATX_RE = re.compile(r"^(#{1,6})\s+(.+?)\s*$", re.MULTILINE)
RFC_PATH_RE = re.compile(r"^docs/rfcs/([0-9]{4})-.*\.md$")
DOGFOOD_RUN_RE = re.compile(r"^docs/dogfood/([0-9]{3})/RUN_SUMMARY\.md$")
DOGFOOD_OP_RE = re.compile(r"^docs/dogfood/([0-9]{3})/OPERATOR_REPORT\.md$")
OPERATOR_DOC_KINDS = {"operator_brief", "work_plan", "progress_note"}


def enumerate_rows(
    conn: Any,
    *,
    repo: Path,
    since_commit: str,
) -> tuple[list[CorpusRow], list[str]]:
    changed = git_helpers.changed_paths(repo, since_commit)
    missing_optional: list[str] = []
    rows: list[CorpusRow] = []
    rows.extend(enumerate_rfcs(repo, changed))
    rows.extend(enumerate_decisions(repo, changed))
    rows.extend(enumerate_operator_docs(repo, changed))
    rows.extend(enumerate_operator_reports(repo, changed))
    rows.extend(enumerate_run_summaries(conn, repo=repo, changed=changed))
    rows.extend(enumerate_audit_chain(conn, repo=repo))
    rows.extend(enumerate_changelog(repo, changed))
    rows.extend(enumerate_ubiquitous_language(repo, changed))
    friction = "docs/HARNESS_FRICTION_PATTERNS.md"
    if (repo / friction).exists():
        rows.extend(enumerate_friction_patterns(repo, changed))
    else:
        missing_optional.append(friction)
    rows.extend(enumerate_commits(repo, since_commit))
    return rows, missing_optional


def enumerate_rfcs(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rows: list[CorpusRow] = []
    for path in sorted((repo / "docs/rfcs").glob("[0-9][0-9][0-9][0-9]-*.md")):
        rel = path.relative_to(repo).as_posix()
        if rel not in changed:
            continue
        match = RFC_PATH_RE.match(rel)
        if match is None:
            continue
        number = match.group(1)
        for heading, body in split_atx_sections(path.read_text(encoding="utf-8")):
            rows.append(_file_row(repo, rel, "rfc", f"rfc:{number}#{slugify(heading)}", f"{heading}\n\n{body}".strip()))
    return rows


def enumerate_decisions(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rel = "docs/DECISION_LOG.md"
    if rel not in changed or not (repo / rel).exists():
        return []
    body = (repo / rel).read_text(encoding="utf-8")
    decisions = _section(body, "## Decisions")
    rows: list[CorpusRow] = []
    for cells in _markdown_table_rows(decisions):
        if len(cells) < 6 or not re.fullmatch(r"D[0-9]{3}", cells[0]):
            continue
        content = "\n\n".join(
            [
                f"Status: {cells[1]}",
                f"Decision: {cells[2]}",
                f"Reason: {cells[3]}",
                f"Consequences: {cells[4]}",
                f"Revisit Trigger: {cells[5]}",
            ]
        )
        rows.append(_file_row(repo, rel, "decision_log_row", f"decision:{cells[0]}", content))
    return rows


def enumerate_operator_reports(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rows: list[CorpusRow] = []
    for path in sorted((repo / "docs/dogfood").glob("[0-9][0-9][0-9]/OPERATOR_REPORT.md")):
        rel = path.relative_to(repo).as_posix()
        if rel not in changed:
            continue
        match = DOGFOOD_OP_RE.match(rel)
        if match is None:
            continue
        dogfood_id = match.group(1)
        text = path.read_text(encoding="utf-8")
        metadata = markdown_front_matter(text)
        blocks = split_operator_report(text)
        for index, block in enumerate(blocks, start=1):
            rows.append(
                _file_row(
                    repo,
                    rel,
                    "operator_report",
                    f"dogfood:{dogfood_id}#intervention-{index}",
                    block,
                    metadata=metadata,
                )
            )
    return rows


def enumerate_operator_docs(repo: Path, changed: set[str]) -> list[CorpusRow]:
    root = repo / "docs/operator"
    if not root.exists():
        return []
    rows: list[CorpusRow] = []
    for path in sorted(root.rglob("*.md")):
        rel = path.relative_to(repo).as_posix()
        if rel not in changed:
            continue
        text = path.read_text(encoding="utf-8")
        metadata = markdown_front_matter(text)
        kind = metadata.get("artifact_kind")
        if not isinstance(kind, str) or kind not in OPERATOR_DOC_KINDS:
            continue
        identifier = _operator_doc_external_id(kind, metadata, rel)
        content = strip_front_matter(text).strip()
        rows.append(_file_row(repo, rel, kind, identifier, content, metadata=metadata))
    return rows


def enumerate_run_summaries(
    conn: Any,
    *,
    repo: Path,
    changed: set[str],
) -> list[CorpusRow]:
    raise NotImplementedError("legacy sqlite run summaries enumeration is retired")


def enumerate_audit_chain(conn: Any, *, repo: Path) -> list[CorpusRow]:
    raise NotImplementedError("legacy sqlite audit chain enumeration is retired")



def enumerate_changelog(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rel = "CHANGELOG.md"
    if rel not in changed or not (repo / rel).exists():
        return []
    text = (repo / rel).read_text(encoding="utf-8")
    rows: list[CorpusRow] = []
    for title, body in _split_by_regex_heading(text, re.compile(r"^##\s+\[?(v[0-9][^\]\s]*)\]?", re.MULTILINE)):
        rows.append(_file_row(repo, rel, "changelog_entry", f"changelog:{title}", body.strip()))
    return rows


def enumerate_ubiquitous_language(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rel = "docs/UBIQUITOUS_LANGUAGE.md"
    if rel not in changed or not (repo / rel).exists():
        return []
    core = _section((repo / rel).read_text(encoding="utf-8"), "## Core Terms")
    rows: list[CorpusRow] = []
    for cells in _markdown_table_rows(core):
        if len(cells) < 2 or cells[0].lower() == "term":
            continue
        rows.append(_file_row(repo, rel, "ubiquitous_language_term", f"ulang:{slugify(cells[0])}", f"{cells[0]}: {cells[1]}"))
    return rows


def enumerate_friction_patterns(repo: Path, changed: set[str]) -> list[CorpusRow]:
    rel = "docs/HARNESS_FRICTION_PATTERNS.md"
    if rel not in changed or not (repo / rel).exists():
        return []
    rows: list[CorpusRow] = []
    for heading, body in split_atx_sections((repo / rel).read_text(encoding="utf-8")):
        if "pattern" not in heading.lower() and "friction" not in heading.lower():
            continue
        rows.append(_file_row(repo, rel, "harness_friction_pattern", f"friction:{slugify(heading)}", f"{heading}\n\n{body}".strip()))
    return rows


def enumerate_commits(repo: Path, since_commit: str) -> list[CorpusRow]:
    rows: list[CorpusRow] = []
    for commit in git_helpers.commits_since(repo, since_commit):
        message = redact_commit_message("\n\n".join(part for part in [commit.subject, commit.body] if part))
        content = json.dumps(
            {
                "sha": commit.sha,
                "author_date": commit.author_date,
                "committer_date": commit.committer_date,
                "parents": list(commit.parents),
                "message": message,
                "changed_paths": list(commit.changed_paths),
            },
            ensure_ascii=False,
            sort_keys=True,
        )
        rows.append(
            CorpusRow(
                external_id=f"commit:{commit.sha}",
                sub_kind="commit",
                content=content,
                provenance=CorpusProvenance(path="<git>", sha256=commit.sha, commit=commit.sha),
                observed_at=commit.committer_date,
            )
        )
    return rows


def split_atx_sections(text: str) -> list[tuple[str, str]]:
    matches = list(ATX_RE.finditer(text))
    sections: list[tuple[str, str]] = []
    for index, match in enumerate(matches):
        start = match.end()
        end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
        title = match.group(2).strip().strip("#").strip()
        body = text[start:end].strip()
        if title and body:
            sections.append((title, body))
    return sections


def split_operator_report(text: str) -> list[str]:
    text = strip_front_matter(text)
    titled = [
        f"{heading}\n\n{body}".strip()
        for heading, body in split_atx_sections(text)
        if "intervention" in heading.lower() or "decision" in heading.lower()
    ]
    if titled:
        return titled
    return [block.strip() for block in re.split(r"\n\s*\n", text) if block.strip() and not block.startswith("---")]


def markdown_front_matter(text: str) -> dict[str, object]:
    block, _body = _split_front_matter(text)
    if block is None:
        return {}
    parsed: dict[str, object] = {}
    for raw_line in block.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        key, sep, raw_value = line.partition(":")
        if not sep:
            continue
        key = key.strip()
        if not key:
            continue
        try:
            parsed[key] = json.loads(raw_value.strip())
        except json.JSONDecodeError:
            continue
    return parsed


def strip_front_matter(text: str) -> str:
    _block, body = _split_front_matter(text)
    return body


def _split_front_matter(text: str) -> tuple[str | None, str]:
    if not text.startswith("---\n"):
        return None, text
    end = text.find("\n---", 4)
    if end == -1:
        return None, text
    body_start = end + 4
    if body_start < len(text) and text[body_start] == "\n":
        body_start += 1
    return text[4:end], text[body_start:]


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "section"


def _operator_doc_external_id(kind: str, metadata: dict[str, object], rel: str) -> str:
    key_by_kind = {
        "operator_brief": "brief_id",
        "work_plan": "plan_id",
        "progress_note": "session_slug",
    }
    raw = metadata.get(key_by_kind[kind])
    if isinstance(raw, str) and raw.strip():
        return f"{kind}:{raw}"
    return f"{kind}:{rel}"


def _file_row(
    repo: Path,
    rel: str,
    sub_kind: str,
    external_id: str,
    content: str,
    *,
    metadata: dict[str, object] | None = None,
) -> CorpusRow:
    validate_source_path(rel)
    commit, observed_at = git_helpers.last_touch(repo, rel)
    metadata = metadata or {}
    return CorpusRow(
        external_id=external_id,
        sub_kind=sub_kind,
        content=content.strip(),
        provenance=CorpusProvenance(path=rel, sha256=git_helpers.file_sha256(repo, rel), commit=commit),
        observed_at=observed_at,
        artifact_kind=_metadata_str(metadata, "artifact_kind"),
        retrieval_priority=_metadata_str(metadata, "retrieval_priority"),
        supersedes=_metadata_str(metadata, "supersedes"),
    )


def _metadata_str(metadata: dict[str, object], key: str) -> str | None:
    value = metadata.get(key)
    return value if isinstance(value, str) and value else None


def _section(text: str, heading: str) -> str:
    start = text.find(heading)
    if start == -1:
        return ""
    next_heading = text.find("\n## ", start + len(heading))
    if next_heading == -1:
        return text[start:]
    return text[start:next_heading]


def _markdown_table_rows(text: str) -> Iterable[list[str]]:
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped.startswith("|") or "---" in stripped:
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if cells and cells[0].lower() not in {"id", "term"}:
            yield cells


def _split_by_regex_heading(text: str, pattern: re.Pattern[str]) -> list[tuple[str, str]]:
    matches = list(pattern.finditer(text))
    sections: list[tuple[str, str]] = []
    for index, match in enumerate(matches):
        start = match.end()
        end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
        sections.append((match.group(1), text[start:end].strip()))
    return sections


def _live_run_ids(conn: Any) -> list[str]:
    try:
        rows = conn.execute("select run_id from runs order by created_at, run_id").fetchall()
    except Exception:  # noqa: BLE001 - corpus export tolerates missing legacy tables.
        return []
    return [str(row["run_id"]) for row in rows]


def _row_by_id(conn: Any, table: str, key_column: str, key_value: str) -> Any:
    row = conn.execute(
        f"SELECT * FROM {table} WHERE {key_column} = ?",
        (key_value,),
    ).fetchone()
    if row is None:
        raise KeyError(f"unknown {table}.{key_column}: {key_value}")
    return row
