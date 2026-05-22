from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def test_compliance_license_issue_verify_prompts_keep_evidence_open() -> None:
    prompt_paths = sorted((ROOT / "docs" / "issues").glob("*/prompts/verify.md"))
    compliance_prompts = [
        path
        for path in prompt_paths
        if "Posture: `compliance_license`" in path.read_text(encoding="utf-8")
    ]

    assert compliance_prompts
    for path in compliance_prompts:
        text = path.read_text(encoding="utf-8")
        assert "scopes findings, not evidence" in text, path
        assert "changed files named by" in text, path
