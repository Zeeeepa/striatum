"""Manifest construction and verification for corpus bundles."""

from __future__ import annotations

import hashlib
import json
import re
from collections.abc import Mapping
from datetime import datetime, timezone
from importlib import metadata
from pathlib import Path

from striatum.corpus import git as git_helpers
from striatum.corpus.types import (
    CORPUS_CONTRACT_VERSION_V2,
    DEFAULT_AUGMENTATION_POLICY,
    DEFAULT_CORPUS_SLUG,
    DEFAULT_REDACTION_TIER,
    DEFAULT_VERIFICATION_DEPTH,
    ROW_SHAPE_VERSION,
    SCHEMA_VERSION,
    SOURCE_KIND,
    SUB_KINDS,
    REDACTION_TIERS,
    SUPPORTED_CORPUS_CONTRACT_VERSIONS,
)
from striatum.errors import StriatumError


def striatum_version() -> str:
    try:
        return metadata.version("striatum-orchestrator")
    except metadata.PackageNotFoundError:
        return "unknown"


def generated_at_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def build_manifest(
    *,
    repo: Path,
    since_ref: str,
    since_commit: str,
    files: dict[str, dict[str, int | str]],
    row_counts: dict[str, int],
    missing_optional_sources: list[str],
    state_authority: Mapping[str, object],
    daemon_audit_included: bool = False,
    generated_at: str | None = None,
    corpus_contract_version: int = CORPUS_CONTRACT_VERSION_V2,
    corpus_slug: str = DEFAULT_CORPUS_SLUG,
    corpus_id: str | None = None,
    redaction_tier: str = DEFAULT_REDACTION_TIER,
    augmentation_policy: Mapping[str, object] | None = None,
    verification_depth: str = DEFAULT_VERIFICATION_DEPTH,
    git_snapshot_hash: str | None = None,
) -> dict[str, object]:
    version = _validate_contract_version(corpus_contract_version)
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "corpus_contract_version": version,
        "striatum_version": striatum_version(),
        "repo_root": str(repo.resolve()),
        "git_head": git_helpers.head_commit(repo),
        "git_dirty": git_helpers.is_dirty(repo),
        "since_ref": since_ref,
        "since_commit": since_commit,
        "generated_at": generated_at or generated_at_now(),
        "schema": {
            "row_shape_version": ROW_SHAPE_VERSION,
            "sub_kinds": list(SUB_KINDS),
        },
        "source_kinds": [SOURCE_KIND],
        "row_counts": {kind: int(row_counts.get(kind, 0)) for kind in SUB_KINDS},
        "files": files,
        "state_authority": _state_authority_payload(state_authority),
        "missing_optional_sources": missing_optional_sources,
        "daemon_audit_included": daemon_audit_included,
    }
    if version >= CORPUS_CONTRACT_VERSION_V2:
        manifest.update(
            {
                "corpus_id": corpus_id or default_corpus_id(repo, slug=corpus_slug),
                "legacy_corpus_alias": SOURCE_KIND,
                "redaction_tier": _validate_redaction_tier(redaction_tier),
                "augmentation_policy": _augmentation_policy_payload(augmentation_policy),
                "verification_depth": _validate_verification_depth(verification_depth),
                "hybrid_archive_defaults": {
                    "snapshot": True,
                    "event_log": True,
                    "verify_replay_by_default": True,
                },
            }
        )
        if git_snapshot_hash:
            manifest["git_snapshot_hash"] = _validate_sha256(git_snapshot_hash, "git_snapshot_hash")
    return manifest


def write_manifest(out: Path, manifest: dict[str, object]) -> tuple[Path, str]:
    path = out / "manifest.json"
    manifest_with_hash = dict(manifest)
    canonical_payload = dict(manifest_with_hash)
    canonical_payload.pop("bundle_sha256", None)
    canonical_payload.pop("generated_at", None)
    canonical = json.dumps(canonical_payload, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")
    digest = hashlib.sha256(canonical).hexdigest()
    manifest_with_hash["bundle_sha256"] = digest
    body = json.dumps(manifest_with_hash, ensure_ascii=False, indent=2, sort_keys=False).encode("utf-8") + b"\n"
    tmp = out / ".manifest.json.tmp"
    tmp.write_bytes(body)
    tmp.replace(path)
    return path, digest


def verify_manifest(manifest: dict[str, object], row_counts: dict[str, int]) -> None:
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise StriatumError("invalid corpus manifest schema_version", exit_code=6)
    try:
        raw_version = int(manifest.get("corpus_contract_version") or 1)
    except (TypeError, ValueError) as exc:
        raise StriatumError("invalid corpus manifest corpus_contract_version", exit_code=6) from exc
    version = _validate_contract_version(raw_version)
    counts = manifest.get("row_counts")
    if not isinstance(counts, dict):
        raise StriatumError("invalid corpus manifest row_counts", exit_code=6)
    for kind in SUB_KINDS:
        if int(counts.get(kind, -1)) != int(row_counts.get(kind, 0)):
            raise StriatumError(f"manifest row count mismatch: {kind}", exit_code=6)
    if version >= CORPUS_CONTRACT_VERSION_V2:
        _verify_v2_manifest_fields(manifest)


def _state_authority_payload(state_authority: Mapping[str, object]) -> dict[str, object]:
    payload = dict(state_authority)
    substrate = payload.get("substrate")
    if not isinstance(substrate, str) or not substrate.strip():
        raise StriatumError("invalid corpus manifest state_authority.substrate", exit_code=6)
    for key in ("daemon_schema_version", "repository_schema_version"):
        if key in payload and payload[key] is not None:
            value = payload[key]
            if not isinstance(value, int | str):
                raise StriatumError(f"invalid corpus manifest state_authority.{key}", exit_code=6)
            payload[key] = int(value)
    return payload


def default_corpus_id(repo: Path, *, slug: str = DEFAULT_CORPUS_SLUG) -> str:
    safe_slug = _slugify(slug or DEFAULT_CORPUS_SLUG)
    origin = _origin_identity(repo)
    return f"{safe_slug}:{hashlib.sha256(origin.encode('utf-8')).hexdigest()}"


def _origin_identity(repo: Path) -> str:
    try:
        origin = git_helpers.git(repo, "config", "--get", "remote.origin.url").strip()
    except StriatumError:
        origin = ""
    if origin:
        return f"git:{origin}"
    return f"file:{repo.resolve()}"


def _slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or DEFAULT_CORPUS_SLUG


def _validate_contract_version(value: int) -> int:
    if value not in SUPPORTED_CORPUS_CONTRACT_VERSIONS:
        raise StriatumError(f"unsupported corpus_contract_version: {value}", exit_code=6)
    return value


def _validate_redaction_tier(value: object) -> str:
    if not isinstance(value, str) or value not in REDACTION_TIERS:
        raise StriatumError("invalid corpus manifest redaction_tier", exit_code=6)
    return value


def _validate_verification_depth(value: object) -> str:
    if value != DEFAULT_VERIFICATION_DEPTH:
        raise StriatumError("invalid corpus manifest verification_depth", exit_code=6)
    return str(value)


def _validate_sha256(value: object, field: str) -> str:
    if not isinstance(value, str) or re.fullmatch(r"[a-f0-9]{64}", value) is None:
        raise StriatumError(f"invalid corpus manifest {field}", exit_code=6)
    return value


def _augmentation_policy_payload(value: Mapping[str, object] | None) -> dict[str, object]:
    payload = dict(DEFAULT_AUGMENTATION_POLICY)
    if value is not None:
        payload.update(value)
    if payload.get("mode") != "reference_only":
        raise StriatumError("invalid corpus manifest augmentation_policy.mode", exit_code=6)
    if payload.get("workflow_opt_in") is not True:
        raise StriatumError("invalid corpus manifest augmentation_policy.workflow_opt_in", exit_code=6)
    if payload.get("required") is not False:
        raise StriatumError("invalid corpus manifest augmentation_policy.required", exit_code=6)
    budget = payload.get("budget_per_packet_lines")
    if not isinstance(budget, int) or budget <= 0:
        raise StriatumError("invalid corpus manifest augmentation_policy.budget_per_packet_lines", exit_code=6)
    return payload


def _verify_v2_manifest_fields(manifest: Mapping[str, object]) -> None:
    corpus_id = manifest.get("corpus_id")
    if not isinstance(corpus_id, str) or re.fullmatch(r"[a-z0-9][a-z0-9-]*:[a-f0-9]{64}", corpus_id) is None:
        raise StriatumError("invalid corpus manifest corpus_id", exit_code=6)
    _validate_redaction_tier(manifest.get("redaction_tier"))
    _validate_verification_depth(manifest.get("verification_depth"))
    policy = manifest.get("augmentation_policy")
    if not isinstance(policy, Mapping):
        raise StriatumError("invalid corpus manifest augmentation_policy", exit_code=6)
    _augmentation_policy_payload(policy)
    hybrid = manifest.get("hybrid_archive_defaults")
    if not isinstance(hybrid, Mapping):
        raise StriatumError("invalid corpus manifest hybrid_archive_defaults", exit_code=6)
    for key in ("snapshot", "event_log", "verify_replay_by_default"):
        if hybrid.get(key) is not True:
            raise StriatumError(f"invalid corpus manifest hybrid_archive_defaults.{key}", exit_code=6)
    if "git_snapshot_hash" in manifest:
        _validate_sha256(manifest.get("git_snapshot_hash"), "git_snapshot_hash")
