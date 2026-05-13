"""Shared types and constants for Striatum corpus exports."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Mapping

SCHEMA_VERSION = "striatum.corpus_export.v1"
ROW_SHAPE_VERSION = "striatum.corpus_row.v1"
SOURCE_KIND = "striatum"

SUB_KINDS: tuple[str, ...] = (
    "rfc",
    "decision_log_row",
    "operator_report",
    "run_summary",
    "audit_chain_entry",
    "changelog_entry",
    "ubiquitous_language_term",
    "harness_friction_pattern",
    "commit",
)

JSONL_FILES: Mapping[str, str] = {
    "rfc": "rfcs.jsonl",
    "decision_log_row": "decision_log_rows.jsonl",
    "operator_report": "operator_reports.jsonl",
    "run_summary": "run_summaries.jsonl",
    "audit_chain_entry": "audit_chain.jsonl",
    "changelog_entry": "changelog.jsonl",
    "ubiquitous_language_term": "ubiquitous_language.jsonl",
    "harness_friction_pattern": "harness_friction_patterns.jsonl",
    "commit": "commits.jsonl",
}


@dataclass(frozen=True)
class CorpusProvenance:
    """Source provenance for one exported corpus row."""

    path: str
    sha256: str
    commit: str

    def to_json(self) -> dict[str, str]:
        return {"path": self.path, "sha256": self.sha256, "commit": self.commit}


@dataclass(frozen=True)
class CorpusRow:
    """One RFC 0044 corpus row."""

    external_id: str
    sub_kind: str
    content: str
    provenance: CorpusProvenance
    observed_at: str

    def to_json(self) -> dict[str, object]:
        return {
            "source_kind": SOURCE_KIND,
            "external_id": self.external_id,
            "sub_kind": self.sub_kind,
            "content": self.content,
            "provenance": self.provenance.to_json(),
            "observed_at": self.observed_at,
        }


@dataclass(frozen=True)
class CorpusBundleResult:
    """Result returned by the export facade."""

    since_ref: str
    since_commit: str
    out: Path
    manifest_path: Path
    row_counts: dict[str, int]
    bundle_sha256: str

    def to_json(self, *, repo: Path) -> dict[str, object]:
        return {
            "status": "exported",
            "since": {"ref": self.since_ref, "commit": self.since_commit},
            "out": _repo_relative(repo, self.out),
            "manifest_path": _repo_relative(repo, self.manifest_path),
            "row_counts": self.row_counts,
            "bundle_sha256": self.bundle_sha256,
        }


def _repo_relative(repo: Path, path: Path) -> str:
    try:
        return path.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError:
        return str(path)
