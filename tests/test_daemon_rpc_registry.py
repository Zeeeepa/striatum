from __future__ import annotations

from typing import Any, cast

from striatum.daemon_rpc.registry import CAPABILITIES, METHOD_REGISTRY, describe_methods


def test_registry_includes_recovery_and_repository_scope_mode() -> None:
    assert "recovery" in CAPABILITIES
    assert "surgical_recovery" in CAPABILITIES
    assert METHOD_REGISTRY["recovery.cancel_job"].required_capability == "recovery"
    assert METHOD_REGISTRY["dogfood.publish_on_behalf"].required_capability == "write"
    assert METHOD_REGISTRY["dogfood.surgical_recovery"].required_capability == "surgical_recovery"
    assert METHOD_REGISTRY["status"].effective_repository_scope_mode == "single_repo"
    assert METHOD_REGISTRY["cross_repo.describe"].effective_repository_scope_mode == "cross_repo"
    assert METHOD_REGISTRY["daemon.token.create"].effective_repository_scope_mode == "daemon_global"

    described = describe_methods()
    methods = cast(list[dict[str, Any]], described["methods"])
    method = next(item for item in methods if item["method"] == "cross_repo.describe")
    assert method["repository_scope_mode"] == "cross_repo"
