"""Daemon RPC method registry loaded from the contract source."""

from __future__ import annotations

import hashlib
import json
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Literal, cast

from striatum.primitives import json_dumps

Capability = Literal["read", "write", "review", "claim", "apply", "admin", "recovery", "surgical_recovery"]
RepositoryScopeMode = Literal["single_repo", "cross_repo", "daemon_global"]
CAPABILITIES: frozenset[str] = frozenset(
    {"read", "write", "review", "claim", "apply", "admin", "recovery", "surgical_recovery"}
)
_REPOSITORY_SCOPE_MODES: frozenset[str] = frozenset(
    {"single_repo", "cross_repo", "daemon_global"}
)
_CONTRACT_SCHEMA_VERSION = 1
_CONTRACT_METHOD_FIELDS: frozenset[str] = frozenset(
    {
        "method",
        "required_capability",
        "repository_scope_mode",
        "params_schema_version",
        "audit_class",
        "min_envelope",
        "deprecated",
    }
)


@dataclass(frozen=True)
class MethodEntry:
    method: str
    required_capability: Capability | None
    repository_scope: bool
    repository_scope_mode: RepositoryScopeMode | None = None
    params_schema_version: int = 1
    audit_class: str = "metadata"
    min_envelope: int = 1
    deprecated: bool = False

    def public_dict(self) -> dict[str, object]:
        return {
            "method": self.method,
            "required_capability": self.required_capability,
            "repository_scope": self.repository_scope,
            "repository_scope_mode": self.effective_repository_scope_mode,
            "params_schema_version": self.params_schema_version,
            "audit_class": self.audit_class,
            "min_envelope": self.min_envelope,
            "deprecated": self.deprecated,
        }

    @property
    def effective_repository_scope_mode(self) -> RepositoryScopeMode:
        if self.repository_scope_mode is not None:
            return self.repository_scope_mode
        return "single_repo" if self.repository_scope else "daemon_global"


def _find_contract_path() -> Path:
    module_path = Path(__file__).resolve()
    for parent in module_path.parents:
        candidate = parent / "contracts" / "daemon_methods.json"
        if candidate.is_file():
            return candidate
    return module_path.parents[3] / "contracts" / "daemon_methods.json"


CONTRACT_PATH = _find_contract_path()


def _load_contract(path: Path = CONTRACT_PATH) -> Mapping[str, object]:
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise RuntimeError(f"daemon method contract not readable: {path}") from exc
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"daemon method contract is invalid JSON: {path}") from exc

    if not isinstance(loaded, dict):
        raise ValueError(f"daemon method contract must be a JSON object: {path}")
    return cast(Mapping[str, object], loaded)


def _entries_from_contract(payload: Mapping[str, object]) -> tuple[MethodEntry, ...]:
    schema_version = payload.get("schema_version")
    if (
        not isinstance(schema_version, int)
        or isinstance(schema_version, bool)
        or schema_version != _CONTRACT_SCHEMA_VERSION
    ):
        raise ValueError(
            "daemon method contract schema_version must be "
            f"{_CONTRACT_SCHEMA_VERSION}"
        )

    methods = payload.get("methods")
    if not isinstance(methods, list):
        raise ValueError("daemon method contract methods must be a list")

    entries: list[MethodEntry] = []
    seen: set[str] = set()
    duplicates: list[str] = []
    for index, method_payload in enumerate(methods):
        if not isinstance(method_payload, dict):
            raise ValueError(f"daemon method contract entry {index} must be an object")
        entry = _entry_from_contract(cast(Mapping[str, object], method_payload), index)
        if entry.method in seen:
            duplicates.append(entry.method)
        seen.add(entry.method)
        entries.append(entry)

    if duplicates:
        raise ValueError(
            "daemon method contract contains duplicate methods: "
            + ", ".join(sorted(duplicates))
        )
    return tuple(entries)


def _entry_from_contract(record: Mapping[str, object], index: int) -> MethodEntry:
    fields = set(record)
    if fields != _CONTRACT_METHOD_FIELDS:
        missing = sorted(_CONTRACT_METHOD_FIELDS - fields)
        unexpected = sorted(fields - _CONTRACT_METHOD_FIELDS)
        details: list[str] = []
        if missing:
            details.append("missing " + ", ".join(missing))
        if unexpected:
            details.append("unexpected " + ", ".join(unexpected))
        raise ValueError(
            f"daemon method contract entry {index} has invalid fields: "
            + "; ".join(details)
        )

    method = _required_str(record, "method", index)
    if not method:
        raise ValueError(f"daemon method contract entry {index} has empty method")
    required_capability = _capability(record["required_capability"], method)
    repository_scope_mode = _repository_scope_mode(record["repository_scope_mode"], method)
    params_schema_version = _positive_int(record, "params_schema_version", method)
    audit_class = _required_str(record, "audit_class", index)
    if not audit_class:
        raise ValueError(f"daemon method contract entry {method!r} has empty audit_class")
    min_envelope = _positive_int(record, "min_envelope", method)
    deprecated = _required_bool(record, "deprecated", method)

    scope_override: RepositoryScopeMode | None = (
        repository_scope_mode if repository_scope_mode == "cross_repo" else None
    )
    return MethodEntry(
        method=method,
        required_capability=required_capability,
        repository_scope=repository_scope_mode == "single_repo",
        repository_scope_mode=scope_override,
        params_schema_version=params_schema_version,
        audit_class=audit_class,
        min_envelope=min_envelope,
        deprecated=deprecated,
    )


def _required_str(record: Mapping[str, object], field: str, index: int) -> str:
    value = record[field]
    if not isinstance(value, str):
        raise ValueError(
            f"daemon method contract entry {index} field {field!r} must be a string"
        )
    return value


def _capability(value: object, method: str) -> Capability | None:
    if value is None:
        return None
    if not isinstance(value, str) or value not in CAPABILITIES:
        raise ValueError(
            f"daemon method contract entry {method!r} has invalid required_capability"
        )
    return cast(Capability, value)


def _repository_scope_mode(value: object, method: str) -> RepositoryScopeMode:
    if not isinstance(value, str) or value not in _REPOSITORY_SCOPE_MODES:
        raise ValueError(
            f"daemon method contract entry {method!r} has invalid repository_scope_mode"
        )
    return cast(RepositoryScopeMode, value)


def _positive_int(record: Mapping[str, object], field: str, method: str) -> int:
    value = record[field]
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        raise ValueError(
            f"daemon method contract entry {method!r} field {field!r} "
            "must be a positive integer"
        )
    return value


def _required_bool(record: Mapping[str, object], field: str, method: str) -> bool:
    value = record[field]
    if not isinstance(value, bool):
        raise ValueError(
            f"daemon method contract entry {method!r} field {field!r} must be a boolean"
        )
    return value


def _methods_etag(entries: tuple[MethodEntry, ...]) -> str:
    material = [entry.public_dict() for entry in sorted(entries, key=lambda item: item.method)]
    return "sha256:" + hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()


_ENTRIES: tuple[MethodEntry, ...] = _entries_from_contract(_load_contract())
METHOD_REGISTRY: dict[str, MethodEntry] = {entry.method: entry for entry in _ENTRIES}
METHODS_ETAG = _methods_etag(_ENTRIES)


def describe_methods() -> dict[str, object]:
    return {
        "methods_etag": METHODS_ETAG,
        "methods": [entry.public_dict() for entry in sorted(_ENTRIES, key=lambda item: item.method)],
    }
