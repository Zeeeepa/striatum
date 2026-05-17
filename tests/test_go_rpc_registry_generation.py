"""Focused tests for the Go daemon RPC registry generator."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


def test_generate_go_rpc_registry_from_contract(tmp_path: Path) -> None:
    repo_root = Path(__file__).resolve().parents[1]
    contract = tmp_path / "daemon_methods.json"
    output = tmp_path / "registry_methods.go"
    contract.write_text(
        json.dumps(
            {
                "methods": [
                    {
                        "method": "daemon.hello",
                        "required_capability": None,
                        "repository_scope_mode": "daemon_global",
                    },
                    {
                        "method": "cross_repo.list",
                        "required_capability": "read",
                        "repository_scope_mode": "cross_repo",
                        "params_schema_version": 1,
                        "audit_class": "metadata",
                        "min_envelope": 1,
                    },
                    {
                        "method": "ack",
                        "required_capability": "claim",
                        "repository_scope_mode": "single_repo",
                        "deprecated": True,
                    },
                ]
            }
        ),
        encoding="utf-8",
    )

    cmd = [
        sys.executable,
        str(repo_root / "scripts" / "generate_go_rpc_registry.py"),
        "--contract",
        str(contract),
        "--out",
        str(output),
    ]
    subprocess.run(cmd, check=True, cwd=repo_root)

    generated = output.read_text(encoding="utf-8")
    assert "var methodEntries = []MethodEntry{" in generated
    assert 'Method: "daemon.hello", RequiredCapability: nil' in generated
    assert 'Method: "cross_repo.list", RequiredCapability: CapPtr(CapabilityRead)' in generated
    assert "RepositoryScopeMode: ScopeCrossRepo" in generated
    assert 'Method: "ack", RequiredCapability: CapPtr(CapabilityClaim)' in generated
    assert "Deprecated: true" in generated

    subprocess.run([*cmd, "--check"], check=True, cwd=repo_root)
