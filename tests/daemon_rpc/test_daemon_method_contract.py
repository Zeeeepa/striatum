from __future__ import annotations

import hashlib
import json
from collections.abc import Mapping
from typing import Any, cast

from striatum.daemon_rpc import registry
from striatum.daemon_rpc.registry import METHOD_REGISTRY, METHODS_ETAG, mcp_tool_descriptor
from striatum.db import json_dumps

EXPECTED_METHOD_FIELDS = {
    "method",
    "required_capability",
    "repository_scope_mode",
    "params_schema_version",
    "audit_class",
    "min_envelope",
    "deprecated",
}
LEGACY_ALIASES = {
    "ack",
    "heartbeat",
    "release",
    "block",
    "complete",
    "publish_artifact",
    "claim_next",
    "verdict",
    "submit_review",
}


def test_daemon_method_contract_file_loads() -> None:
    payload = _contract_payload()

    assert registry.CONTRACT_PATH.name == "daemon_methods.json"
    assert payload["schema_version"] == 1
    assert _method_records()

    for index, record in enumerate(_method_records()):
        assert set(record) == EXPECTED_METHOD_FIELDS, index
        assert isinstance(record["method"], str)
        assert record["repository_scope_mode"] in {
            "single_repo",
            "cross_repo",
            "daemon_global",
        }
        assert record["params_schema_version"] == 1
        assert record["audit_class"] == "metadata"
        assert record["min_envelope"] == 1
        assert isinstance(record["deprecated"], bool)


def test_every_contract_method_appears_in_method_registry() -> None:
    contract_methods = {record["method"] for record in _method_records()}

    assert contract_methods == set(METHOD_REGISTRY)


def test_deprecated_aliases_are_preserved_from_contract() -> None:
    contract_deprecated = {
        record["method"] for record in _method_records() if record["deprecated"]
    }
    registry_deprecated = {
        method for method, entry in METHOD_REGISTRY.items() if entry.deprecated
    }

    assert LEGACY_ALIASES <= contract_deprecated
    assert "recovery.auto_publish_stale_artifacts" in contract_deprecated
    assert contract_deprecated == registry_deprecated


def test_methods_etag_is_derived_from_contract_content() -> None:
    material = [_public_dict_from_contract(record) for record in _method_records()]
    material.sort(key=lambda record: cast(str, record["method"]))
    expected = "sha256:" + hashlib.sha256(
        json_dumps(material).encode("utf-8")
    ).hexdigest()

    assert METHODS_ETAG == expected
    assert registry.describe_methods()["methods_etag"] == expected


def test_contract_loader_preserves_effective_scope_behavior() -> None:
    assert METHOD_REGISTRY["status"].repository_scope is True
    assert METHOD_REGISTRY["status"].repository_scope_mode is None
    assert METHOD_REGISTRY["status"].effective_repository_scope_mode == "single_repo"
    assert METHOD_REGISTRY["daemon.shutdown"].repository_scope is False
    assert METHOD_REGISTRY["daemon.shutdown"].repository_scope_mode is None
    assert (
        METHOD_REGISTRY["daemon.shutdown"].effective_repository_scope_mode
        == "daemon_global"
    )
    assert METHOD_REGISTRY["cross_repo.describe"].repository_scope is False
    assert METHOD_REGISTRY["cross_repo.describe"].repository_scope_mode == "cross_repo"


def test_escalation_methods_are_declared_with_principal_capabilities() -> None:
    assert METHOD_REGISTRY["escalation.list"].required_capability == "read"
    assert METHOD_REGISTRY["escalation.show"].required_capability == "read"
    assert METHOD_REGISTRY["escalation.resolve"].required_capability == "admin"
    assert METHOD_REGISTRY["escalation.resolve"].effective_repository_scope_mode == "single_repo"


def test_auto_finalize_method_is_recovery_scoped_and_not_recovery_auto() -> None:
    assert METHOD_REGISTRY["recovery.auto_finalize"].required_capability == "recovery"
    assert (
        METHOD_REGISTRY["recovery.auto_finalize"].effective_repository_scope_mode
        == "single_repo"
    )
    assert "recovery.auto_finalize" in METHOD_REGISTRY
    assert not METHOD_REGISTRY["recovery.auto_finalize"].deprecated


def test_mcp_tool_descriptor_is_derived_from_method_entry() -> None:
    assert mcp_tool_descriptor(METHOD_REGISTRY["status"]) == {
        "name": "status",
        "description": "Invoke daemon RPC method `status`.",
        "required_capability": "read",
        "repository_scope_mode": "single_repo",
    }
    assert mcp_tool_descriptor(METHOD_REGISTRY["dogfood.surgical_recovery"]) == {
        "name": "dogfood.surgical_recovery",
        "description": "Invoke daemon RPC method `dogfood.surgical_recovery`.",
        "required_capability": "surgical_recovery",
        "repository_scope_mode": "single_repo",
    }


def _contract_payload() -> dict[str, Any]:
    loaded = json.loads(registry.CONTRACT_PATH.read_text(encoding="utf-8"))
    assert isinstance(loaded, dict)
    return cast(dict[str, Any], loaded)


def _method_records() -> list[Mapping[str, Any]]:
    methods = _contract_payload()["methods"]
    assert isinstance(methods, list)
    return [cast(Mapping[str, Any], item) for item in methods]


def _public_dict_from_contract(record: Mapping[str, Any]) -> dict[str, object]:
    scope_mode = cast(str, record["repository_scope_mode"])
    return {
        "method": record["method"],
        "required_capability": record["required_capability"],
        "repository_scope": scope_mode == "single_repo",
        "repository_scope_mode": scope_mode,
        "params_schema_version": record["params_schema_version"],
        "audit_class": record["audit_class"],
        "min_envelope": record["min_envelope"],
        "deprecated": record["deprecated"],
    }
