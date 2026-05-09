"""RFC 0025 V1 Step 1: claude_code plugin install tests."""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from striatum.errors import InvalidTransitionError
from striatum.plugins.install import install, uninstall


CLAUDE_CODE_FILES = {
    ".claude-plugin/plugin.json",
    "skills/striatum-workflow/SKILL.md",
    "skills/striatum-scaffold/SKILL.md",
    "skills/striatum-claim-loop/SKILL.md",
    "skills/striatum-supervise/SKILL.md",
    "skills/striatum-recover/SKILL.md",
    "commands/claim-next.md",
    "commands/status.md",
    "commands/why.md",
    "commands/dashboard.md",
    "commands/doctor.md",
    "hooks/hooks.json",
    ".mcp.json",
    "README.md",
}


def test_claude_code_install_writes_full_bundle(tmp_path: Path) -> None:
    res = install(target=tmp_path, profile="claude_code")
    rel_paths = {f["path"] for f in res["files"]}
    assert rel_paths == CLAUDE_CODE_FILES
    bundle = tmp_path / ".striatum" / "plugins" / "claude_code"
    for rel in CLAUDE_CODE_FILES:
        assert (bundle / rel).is_file(), rel
    assert (bundle / ".manifest.json").is_file()


def test_claude_code_idempotent_re_install(tmp_path: Path) -> None:
    """Second install is a byte-no-op on every bundle file (manifest excluded per F4)."""
    install(target=tmp_path, profile="claude_code")
    bundle = tmp_path / ".striatum" / "plugins" / "claude_code"
    snapshots = {rel: (bundle / rel).read_bytes() for rel in CLAUDE_CODE_FILES}
    install(target=tmp_path, profile="claude_code")
    for rel, before in snapshots.items():
        after = (bundle / rel).read_bytes()
        assert before == after, rel
    second = install(target=tmp_path, profile="claude_code")
    for f in second["files"]:
        assert f["status"] == "skipped_unchanged", f


def test_claude_code_dry_run_writes_nothing(tmp_path: Path) -> None:
    res = install(target=tmp_path, profile="claude_code", dry_run=True)
    bundle = tmp_path / ".striatum" / "plugins" / "claude_code"
    assert not bundle.exists()
    assert all(f["status"] == "dry_run" for f in res["files"])


def test_claude_code_edit_detect_refuses_overwrite(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    edited = tmp_path / ".striatum" / "plugins" / "claude_code" / "commands" / "claim-next.md"
    edited.write_text("OPERATOR EDIT", encoding="utf-8")
    res = install(target=tmp_path, profile="claude_code")
    statuses = {f["path"]: f["status"] for f in res["files"]}
    assert statuses["commands/claim-next.md"] == "refused_modified"
    assert edited.read_text(encoding="utf-8") == "OPERATOR EDIT"


def test_claude_code_force_overwrites(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    edited = tmp_path / ".striatum" / "plugins" / "claude_code" / "commands" / "claim-next.md"
    edited.write_text("OPERATOR EDIT", encoding="utf-8")
    res = install(target=tmp_path, profile="claude_code", force=True)
    statuses = {f["path"]: f["status"] for f in res["files"]}
    assert statuses["commands/claim-next.md"] == "written"
    assert edited.read_text(encoding="utf-8") != "OPERATOR EDIT"


def test_claude_code_no_external_urls(tmp_path: Path) -> None:
    """F3: rendered output must not embed external URLs / source-repo paths / home."""
    install(target=tmp_path, profile="claude_code")
    bundle = tmp_path / ".striatum" / "plugins" / "claude_code"
    forbidden = re.compile(r"(?:https?|git|file|ssh|ftp)://")
    for rel in CLAUDE_CODE_FILES:
        body = (bundle / rel).read_text(encoding="utf-8")
        assert not forbidden.search(body), f"{rel} contains a URL scheme"
        assert "/home/" not in body, f"{rel} leaks home directory"
        # Allow the literal string ".striatum" — it's our own namespace.


def test_claude_code_marketplace_appended(tmp_path: Path) -> None:
    res = install(target=tmp_path, profile="claude_code")
    assert res["marketplace"] is not None
    mp = json.loads((tmp_path / ".striatum" / "plugins" / "marketplace.json").read_text())
    assert mp["name"] == "local-striatum"
    plugins = mp["plugins"]
    assert len(plugins) == 1
    assert plugins[0]["source"]["path"] == "./claude_code"


def test_claude_code_marketplace_idempotent(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    install(target=tmp_path, profile="claude_code")
    mp = json.loads((tmp_path / ".striatum" / "plugins" / "marketplace.json").read_text())
    plugins = mp["plugins"]
    assert len(plugins) == 1, plugins  # not duplicated


def test_uninstall_removes_tracked_files(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    bundle = tmp_path / ".striatum" / "plugins" / "claude_code"
    assert bundle.exists()
    res = uninstall(target=tmp_path, profile="claude_code")
    assert res["status"] == "uninstalled"
    assert not bundle.exists()


def test_uninstall_refuses_modified_without_force(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    edited = tmp_path / ".striatum" / "plugins" / "claude_code" / "README.md"
    edited.write_text("hand-edited", encoding="utf-8")
    res = uninstall(target=tmp_path, profile="claude_code")
    assert res["status"] == "refused"
    assert edited.exists()


def test_uninstall_force_removes_modified(tmp_path: Path) -> None:
    install(target=tmp_path, profile="claude_code")
    edited = tmp_path / ".striatum" / "plugins" / "claude_code" / "README.md"
    edited.write_text("hand-edited", encoding="utf-8")
    res = uninstall(target=tmp_path, profile="claude_code", force=True)
    assert res["status"] == "uninstalled"
    assert not edited.exists()


def test_unknown_profile_raises(tmp_path: Path) -> None:
    with pytest.raises(InvalidTransitionError):
        install(target=tmp_path, profile="codex")  # Step 2


def test_unknown_scope_raises(tmp_path: Path) -> None:
    with pytest.raises(InvalidTransitionError):
        install(target=tmp_path, profile="claude_code", scope="global")


def test_skill_templates_match_skills_module(tmp_path: Path) -> None:
    """F1: skill bodies in plugins/templates must match skills/templates byte-for-byte."""
    repo_root = Path(__file__).resolve().parent.parent
    for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover"):
        skills_path = repo_root / "src/striatum/skills/templates/claude_code" / f"{skill}.md.tmpl"
        plugins_path = repo_root / "src/striatum/plugins/templates/claude_code/skills" / f"{skill}.md.tmpl"
        assert skills_path.read_bytes() == plugins_path.read_bytes(), skill
