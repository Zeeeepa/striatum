"""Artifact file and presentation helpers for the local web UI."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Sequence

__all__ = [
    "ArtifactRawContext",
    "ArtifactViewPayloadError",
    "InlineArtifactBody",
    "artifact_content_type",
    "artifact_view_template_context",
    "byline_line",
    "inline_artifact_body",
    "lane_evidence_chip",
    "recorded_artifact_attestation_chip",
    "resolve_artifact_file",
    "serve_artifact_raw",
    "shape_artifact_rows",
]

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
ResponseStarter = Callable[[int], None]
HeaderSender = Callable[[str, str], None]
HeaderFinisher = Callable[[], None]
BodyWriter = Callable[[bytes], object]


class ArtifactViewPayloadError(ValueError):
    """Raised when the daemon artifact-view DTO is missing required fields."""


@dataclass(frozen=True)
class ArtifactRawContext:
    repo: Path
    send_json: JsonSender
    send_response: ResponseStarter
    send_header: HeaderSender
    end_headers: HeaderFinisher
    write_body: BodyWriter


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


def serve_artifact_raw(ctx: ArtifactRawContext, artifact_id: str) -> None:
    """Stream raw artifact bytes for the web UI viewer."""

    if not artifact_id:
        ctx.send_json(400, _error(400, "missing artifact id"))
        return
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(ctx.repo, "artifact.show", {"artifact_id": artifact_id})
        artifact_raw = payload.get("artifact")
        if not isinstance(artifact_raw, Mapping):
            ctx.send_json(
                500,
                _error(500, "daemon artifact DTO missing artifact"),
            )
            return
        artifact = artifact_raw
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return

    # RFC 0072: prefer the daemon-fetched body when the artifact is
    # blob-routed (blob_key set). Otherwise fall back to the legacy
    # disk read.
    data: bytes | None = None
    content_type: str | None = None
    blob_key = artifact.get("blob_key")
    if isinstance(blob_key, str) and blob_key:
        try:
            payload = call_repo_method(
                ctx.repo,
                "artifact.get_content",
                {"artifact_id": artifact_id},
            )
        except ServiceDaemonRpcError as exc:
            ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
            return
        except Exception as exc:  # noqa: BLE001
            ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
            return
        body_base64 = payload.get("body_base64") if isinstance(payload, Mapping) else None
        if not isinstance(body_base64, str):
            ctx.send_json(500, _error(500, "blob artifact missing body_base64"))
            return
        try:
            import base64

            data = base64.b64decode(body_base64, validate=True)
        except (ValueError, TypeError) as exc:
            ctx.send_json(500, _error(500, f"blob body base64 decode failed: {exc}"))
            return
        content_type_raw = payload.get("content_type") if isinstance(payload, Mapping) else None
        if isinstance(content_type_raw, str) and content_type_raw:
            content_type = content_type_raw

    if data is None:
        try:
            repo_path = resolve_artifact_file(ctx.repo, artifact.get("repo_path"))
        except ValueError:
            ctx.send_json(404, _error(404, "artifact file missing on disk"))
            return
        if not repo_path.is_file():
            ctx.send_json(404, _error(404, "artifact file missing on disk"))
            return
        try:
            data = repo_path.read_bytes()
        except OSError as exc:
            ctx.send_json(500, _error(500, str(exc)))
            return
        if content_type is None:
            content_type = artifact_content_type(repo_path)
    if content_type is None:
        content_type = "application/octet-stream"
    ctx.send_response(200)
    ctx.send_header("Content-Type", content_type)
    ctx.send_header("Content-Length", str(len(data)))
    ctx.send_header("Content-Security-Policy", "default-src 'none'")
    ctx.send_header("Connection", "close")
    ctx.end_headers()
    try:
        ctx.write_body(data)
    except BrokenPipeError:
        return


def inline_artifact_body(repo: Path, artifact: Mapping[str, Any]) -> InlineArtifactBody:
    # RFC 0072: blob-routed artifacts have their body in S3, not on
    # disk. Fetch via daemon RPC artifact.get_content; the daemon
    # verifies sha256 on read. Markdown bodies render through the same
    # md pipeline as the disk path; non-markdown returns no inline
    # body (operator opens the raw endpoint).
    blob_key = artifact.get("blob_key")
    if isinstance(blob_key, str) and blob_key:
        return _inline_blob_artifact_body(repo, artifact)

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


def _inline_blob_artifact_body(repo: Path, artifact: Mapping[str, Any]) -> InlineArtifactBody:
    artifact_id = artifact.get("artifact_id")
    repo_path = artifact.get("repo_path")
    if not isinstance(artifact_id, str) or not artifact_id:
        return InlineArtifactBody()
    if not isinstance(repo_path, str) or not repo_path.endswith(".md"):
        # Non-markdown bodies (.json, binary, etc.) do not get inline
        # rendering. The raw endpoint serves them.
        return InlineArtifactBody()

    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        payload = call_repo_method(
            repo,
            "artifact.get_content",
            {"artifact_id": artifact_id},
        )
    except ServiceDaemonRpcError:
        return InlineArtifactBody()
    except Exception:  # noqa: BLE001
        return InlineArtifactBody()

    body_base64 = payload.get("body_base64") if isinstance(payload, Mapping) else None
    if not isinstance(body_base64, str):
        return InlineArtifactBody()
    try:
        import base64

        body_bytes = base64.b64decode(body_base64, validate=True)
    except (ValueError, TypeError):
        return InlineArtifactBody()
    try:
        body_text = body_bytes.decode("utf-8")
    except UnicodeDecodeError:
        return InlineArtifactBody()

    from striatum.web.markdown import render as md_render

    return InlineArtifactBody(rendered_md=md_render(body_text), body_text=None)


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


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


def _rpc_error(code: str, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}
