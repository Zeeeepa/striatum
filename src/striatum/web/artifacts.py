"""Artifact file and presentation helpers for the local web UI."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Sequence

__all__ = [
    "ArtifactViewPayloadError",
    "InlineArtifactBody",
    "artifact_content_type",
    "artifact_view_template_context",
    "byline_line",
    "inline_artifact_body",
    "lane_evidence_chip",
    "recorded_artifact_attestation_chip",
    "resolve_artifact_file",
    "shape_artifact_rows",
]


class ArtifactViewPayloadError(ValueError):
    """Raised when the daemon artifact-view DTO is missing required fields."""


@dataclass(frozen=True)
class InlineArtifactBody:
    rendered_md: str | None = None
    body_text: str | None = None


def resolve_artifact_file(repo: Path, repo_path: object) -> Path:
    if not isinstance(repo_path, str) or not repo_path:
        raise ValueError("artifact repo_path must be a non-empty string")
    relative = Path(repo_path)
    if relative.is_absolute():
        raise ValueError("artifact repo_path must be repository-relative")
    full = (repo / relative).resolve()
    full.relative_to(repo.resolve())
    return full


def artifact_content_type(path: Path) -> str:
    suffix = path.suffix.lower()
    return {
        ".md": "text/markdown; charset=utf-8",
        ".markdown": "text/markdown; charset=utf-8",
        ".json": "application/json",
        ".txt": "text/plain; charset=utf-8",
    }.get(suffix, "application/octet-stream")


def inline_artifact_body(repo: Path, artifact: Mapping[str, Any]) -> InlineArtifactBody:
    repo_path = artifact.get("repo_path") or ""
    try:
        full = resolve_artifact_file(repo, repo_path)
        if not isinstance(repo_path, str) or not repo_path.endswith(".md") or not full.is_file():
            return InlineArtifactBody()
        from striatum.web.markdown import render as md_render

        body = full.read_text(encoding="utf-8", errors="replace")
        return InlineArtifactBody(rendered_md=md_render(body), body_text=None)
    except (ValueError, OSError):
        return InlineArtifactBody()


def artifact_view_template_context(
    repo: Path,
    payload: Mapping[str, Any],
) -> dict[str, Any]:
    """Return the Jinja context for ``artifact_view.html`` from a daemon DTO."""

    run_raw = payload.get("run")
    artifact_raw = payload.get("artifact")
    if not isinstance(run_raw, Mapping) or not isinstance(artifact_raw, Mapping):
        raise ArtifactViewPayloadError("daemon artifact DTO missing fields")

    run = dict(run_raw)
    artifact = dict(artifact_raw)
    if "lane_attestation_chip" not in artifact:
        expected_rows = [
            {
                "path": artifact.get("repo_path"),
                "expected_author_line": payload.get("expected_author_line"),
            }
        ]
        shaped = shape_artifact_rows(
            artifacts=[artifact],
            expected_rows=expected_rows,
        )
        if shaped:
            artifact = dict(shaped[0])
    if "provenance_trail" not in artifact:
        trail = payload.get("provenance_trail")
        artifact["provenance_trail"] = trail if isinstance(trail, list) else []
    body = inline_artifact_body(repo, artifact)
    return {
        "run": run,
        "artifact": artifact,
        "rendered_md": body.rendered_md,
        "body_text": body.body_text,
    }


def recorded_artifact_attestation_chip(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attestation_override_rationale: Any = None,
) -> dict[str, Any]:
    actual = str(author_line).strip().lower() if author_line else ""
    expected = (
        str(expected_author_line).strip().lower()
        if expected_author_line is not None
        else ""
    )
    if attestation_override_rationale:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_override",
            "supervisor_id": None,
            "label": "unattested",
        }
    if actual.startswith("author: operator"):
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_byline",
            "supervisor_id": None,
            "label": "unattested",
        }
    if expected and actual == expected:
        return {
            "state": "attested",
            "attested": True,
            "reason": None,
            "supervisor_id": None,
            "label": "attested",
        }
    return {
        "state": "unattested",
        "attested": False,
        "reason": "expected_author_line_mismatch" if expected else "expected_author_line_missing",
        "supervisor_id": None,
        "label": "unattested",
    }


def lane_evidence_chip(*, attestation_override_rationale: Any = None) -> dict[str, Any]:
    rationale = (
        str(attestation_override_rationale).strip()
        if attestation_override_rationale is not None
        else ""
    )
    if rationale:
        return {
            "state": "override",
            "label": "override",
            "muted": False,
            "rationale": rationale,
        }
    return {
        "state": "not_yet_correlated",
        "label": "not yet correlated",
        "muted": True,
        "rationale": None,
    }


def byline_line(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attested: bool | None = None,
    operator_label: Any = None,
) -> dict[str, Any]:
    actual = str(author_line) if author_line is not None else None
    expected = str(expected_author_line) if expected_author_line is not None else None
    if attested is False:
        label = str(operator_label).strip() if operator_label else ""
        display = f"author: operator [self-declared: {label}]" if label else "author: operator"
    else:
        display = actual if actual else "author: <missing>"
    return {
        "author_line": actual,
        "expected_author_line": expected,
        "display": display,
        "attested": attested,
        "matches_expected": (
            None if actual is None or expected is None else actual == expected
        ),
    }


def shape_artifact_rows(
    *,
    artifacts: list[dict[str, Any]],
    expected_rows: Sequence[Mapping[str, Any]],
) -> list[dict[str, Any]]:
    expected_by_path = {str(row.get("path")): row for row in expected_rows}
    shaped: list[dict[str, Any]] = []
    for artifact in artifacts:
        row = dict(artifact)
        expected = expected_by_path.get(str(row.get("repo_path")))
        expected_author_line = expected.get("expected_author_line") if expected else None
        override_rationale = row.get("attestation_override_rationale")
        attestation = recorded_artifact_attestation_chip(
            row.get("author_line"),
            expected_author_line=expected_author_line,
            attestation_override_rationale=override_rationale,
        )
        row["expected_author_line"] = expected_author_line
        row["byline_line"] = byline_line(
            row.get("author_line"),
            expected_author_line=expected_author_line,
            attested=bool(attestation.get("attested")),
        )
        row["lane_attestation_chip"] = attestation
        row["lane_evidence_chip"] = lane_evidence_chip(
            attestation_override_rationale=override_rationale,
        )
        row["attestation_override_rationale"] = override_rationale
        shaped.append(row)
    return shaped
