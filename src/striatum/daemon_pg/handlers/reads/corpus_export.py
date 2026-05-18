"""PG handler for ``corpus.export``."""

from __future__ import annotations

import json
from collections.abc import Mapping
from typing import Any

from striatum.cli.run_summary import render_run_summary_markdown
from striatum.corpus import git as git_helpers
from striatum.corpus.enumerator import (
    enumerate_changelog,
    enumerate_commits,
    enumerate_decisions,
    enumerate_friction_patterns,
    enumerate_operator_docs,
    enumerate_operator_reports,
    enumerate_rfcs,
)
from striatum.corpus.manifest import build_manifest, verify_manifest, write_manifest
from striatum.corpus.redaction import redact_event_payload, redact_run_summary_payload
from striatum.corpus.types import SUB_KINDS, CorpusBundleResult, CorpusProvenance, CorpusRow
from striatum.corpus.writer import verify_jsonl_files, write_jsonl_bundle
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._read_model import events_for
from ._registry import register_pg_handler
from ._sql import require_text, safe_output_path
from .run_summary import _run_row, run_summary_payload


@register_pg_handler("corpus.export", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    since = require_text(params, "since")
    out_text = require_text(params, "out")
    try:
        since_commit = git_helpers.resolve_commit(ctx.repo_root, since)
    except Exception as exc:
        raise RpcError("not_found", str(exc)) from exc
    out = safe_output_path(ctx, out_text)
    if out.exists() and not out.is_dir():
        raise RpcError("path_outside_scope", "--out must be a directory", exit_code=8)
    changed = git_helpers.changed_paths(ctx.repo_root, since_commit)
    missing_optional: list[str] = []
    rows: list[CorpusRow] = []
    rows.extend(enumerate_rfcs(ctx.repo_root, changed))
    rows.extend(enumerate_decisions(ctx.repo_root, changed))
    rows.extend(enumerate_operator_docs(ctx.repo_root, changed))
    rows.extend(enumerate_operator_reports(ctx.repo_root, changed))
    rows.extend(_enumerate_run_summaries(ctx))
    rows.extend(_enumerate_audit_chain(ctx))
    rows.extend(enumerate_changelog(ctx.repo_root, changed))
    friction = "docs/HARNESS_FRICTION_PATTERNS.md"
    if (ctx.repo_root / friction).exists():
        rows.extend(enumerate_friction_patterns(ctx.repo_root, changed))
    else:
        missing_optional.append(friction)
    rows.extend(enumerate_commits(ctx.repo_root, since_commit))

    files = write_jsonl_bundle(out, rows)
    row_counts = verify_jsonl_files(out, files)
    manifest = build_manifest(
        repo=ctx.repo_root,
        since_ref=since,
        since_commit=since_commit,
        files=files,
        row_counts={kind: row_counts.get(kind, 0) for kind in SUB_KINDS},
        missing_optional_sources=missing_optional,
        state_authority=_manifest_state_authority(ctx),
        daemon_audit_included=True,
    )
    verify_manifest(manifest, row_counts)
    manifest_path, bundle_sha256 = write_manifest(out, manifest)
    return CorpusBundleResult(
        since_ref=since,
        since_commit=since_commit,
        out=out,
        manifest_path=manifest_path,
        row_counts={kind: row_counts.get(kind, 0) for kind in SUB_KINDS},
        bundle_sha256=bundle_sha256,
    ).to_json(repo=ctx.repo_root)


def _enumerate_run_summaries(ctx: RepoHandlerContext) -> list[CorpusRow]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.created_at
            FROM striatumd.runs r
            WHERE r.repository_id = %s
            ORDER BY r.created_at, r.run_id
            """,
            (ctx.repository_id,),
        )
        runs = [dict(row) for row in cur.fetchall()]
    result: list[CorpusRow] = []
    for run in runs:
        run_id = str(run["run_id"])
        run_row = _run_row(ctx, run_id)
        summary = run_summary_payload(ctx, run_id=run_id, run=run_row)
        content = render_run_summary_markdown(run=run_row, summary=redact_run_summary_payload(summary))
        result.append(
            CorpusRow(
                external_id=f"run:{run_id}",
                sub_kind="run_summary",
                content=content,
                provenance=CorpusProvenance(
                    path="<daemon-postgres>",
                    sha256="daemon-postgres",
                    commit=git_helpers.head_commit(ctx.repo_root),
                ),
                observed_at=str(run["created_at"]),
            )
        )
    return result


def _enumerate_audit_chain(ctx: RepoHandlerContext) -> list[CorpusRow]:
    result: list[CorpusRow] = []
    for event in events_for(ctx, limit=10000):
        payload = {
            "event_id": event.get("event_id"),
            "event_type": event.get("event_type"),
            "run_id": event.get("run_id"),
            "job_id": event.get("job_id"),
            "created_at": event.get("created_at"),
        }
        raw = event.get("payload_json")
        if isinstance(raw, dict):
            payload.update(redact_event_payload(raw))
        content = json.dumps(redact_event_payload(payload), ensure_ascii=False, sort_keys=True)
        result.append(
            CorpusRow(
                external_id=f"audit:repo:{event['event_id']}",
                sub_kind="audit_chain_entry",
                content=content,
                provenance=CorpusProvenance(
                    path="<daemon-postgres>",
                    sha256="daemon-postgres",
                    commit=git_helpers.head_commit(ctx.repo_root),
                ),
                observed_at=str(event["created_at"]),
            )
        )
    return result


def _manifest_state_authority(ctx: RepoHandlerContext) -> dict[str, object]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.repository_id,
                   r.last_schema_version AS repository_schema_version,
                   sm.value AS daemon_schema_version
            FROM striatumd.repositories r
            LEFT JOIN striatumd.schema_meta sm
              ON sm.key = 'substrate_version'
            WHERE r.repository_id = %s
            """,
            (ctx.repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"repository not found: {ctx.repository_id}")
    return {
        "substrate": "postgresql",
        "repository_id": str(row["repository_id"]),
        "repository_schema_version": int(row["repository_schema_version"]),
        "daemon_schema_version": int(row["daemon_schema_version"] or 0),
    }
