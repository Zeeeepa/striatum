from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT / "src") not in sys.path:
    sys.path.insert(0, str(ROOT / "src"))

from striatum import __version__ as STRIATUM_VERSION  # noqa: E402
from striatum.skills import install as install_skills  # noqa: E402
from striatum.skills import (  # noqa: E402
    bundled_template_sha256,
    load_manifest,
    manifest_path_for,
    skill_files_present,
)


def _claude_paths(target: Path, namespace: str = "striatum-") -> list[Path]:
    return [
        target / ".claude" / "skills" / f"{namespace}{skill}" / "SKILL.md"
        for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp")
    ]


def test_install_claude_code_writes_skills_and_manifest(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="claude_code")
    assert result["profile"] == "claude_code"
    assert {f["status"] for f in result["files"]} == {"written"}
    for path in _claude_paths(tmp_path):
        assert path.exists(), path
    manifest_path = Path(result["manifest_path"])
    manifest = load_manifest(manifest_path)
    assert manifest is not None
    assert manifest["schema_version"] == "striatum.skills.manifest.v1"
    assert manifest["striatum_version"] == STRIATUM_VERSION
    assert {entry["path"] for entry in manifest["files"]} == {
        f".claude/skills/striatum-{skill}/SKILL.md"
        for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp")
    }


def test_install_idempotent_byte_identical(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    snapshot = {
        path: path.read_bytes()
        for path in _claude_paths(tmp_path)
    }
    second = install_skills(target=tmp_path, profile="claude_code")
    assert {f["status"] for f in second["files"]} == {"skipped_unchanged"}
    for path, data in snapshot.items():
        assert path.read_bytes() == data


def test_install_refuses_modified_file_without_force(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    target_skill = tmp_path / ".claude/skills/striatum-claim-loop/SKILL.md"
    edited = target_skill.read_text(encoding="utf-8") + "\n# operator note\n"
    target_skill.write_text(edited, encoding="utf-8")
    result = install_skills(target=tmp_path, profile="claude_code")
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert (
        statuses[".claude/skills/striatum-claim-loop/SKILL.md"]
        == "refused_modified"
    )
    # Operator edit preserved verbatim.
    assert target_skill.read_text(encoding="utf-8") == edited


def test_install_force_overwrites_modified_file(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    target_skill = tmp_path / ".claude/skills/striatum-claim-loop/SKILL.md"
    target_skill.write_text("hand-edit\n", encoding="utf-8")
    result = install_skills(target=tmp_path, profile="claude_code", force=True)
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert (
        statuses[".claude/skills/striatum-claim-loop/SKILL.md"] == "written"
    )
    assert "hand-edit" not in target_skill.read_text(encoding="utf-8")


def test_install_dry_run_writes_nothing(tmp_path: Path) -> None:
    fresh = tmp_path / "first"
    fresh.mkdir()
    result = install_skills(target=fresh, profile="claude_code", dry_run=True)
    assert {f["status"] for f in result["files"]} == {"dry_run"}
    for path in _claude_paths(fresh):
        assert not path.exists()
    # Manifest is also not written under dry-run.
    assert not Path(result["manifest_path"]).exists()


def test_install_generic_profile_single_guide(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="generic")
    paths = [f["path"] for f in result["files"]]
    assert paths == ["striatum-STRIATUM_AGENT_GUIDE.md"]
    guide = tmp_path / "striatum-STRIATUM_AGENT_GUIDE.md"
    assert guide.exists()
    manifest = load_manifest(Path(result["manifest_path"]))
    assert manifest is not None
    assert manifest["profile"] == "generic"


def test_install_namespace_prefix_respected(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code", namespace="foo-")
    expected = tmp_path / ".claude/skills/foo-workflow/SKILL.md"
    assert expected.exists(), list(tmp_path.glob(".claude/skills/*"))


def test_install_scope_user_writes_to_home(tmp_path: Path) -> None:
    fake_home = tmp_path / "home"
    fake_home.mkdir()
    result = install_skills(
        target=tmp_path, profile="claude_code", scope="user", home=fake_home
    )
    assert result["scope"] == "user"
    expected = fake_home / ".claude/skills/striatum-workflow/SKILL.md"
    assert expected.exists()


def test_init_with_skills_installs_after_init(tmp_path: Path) -> None:
    import subprocess

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    proc = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "init",
            "--with-skills",
            "claude_code",
            "--json",
        ],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 0, proc.stderr
    payload = json.loads(proc.stdout)
    assert payload["ok"] is True
    assert payload["data"]["state_store"] == "daemon_postgres"
    assert (tmp_path / ".striatum").is_dir()
    assert not (tmp_path / ".striatum" / "state.sqlite3").exists()
    assert (tmp_path / ".claude/skills/striatum-workflow/SKILL.md").exists()


def test_skill_files_present_reports_missing_claude_skill(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    target = tmp_path / ".claude/skills/striatum-recover/SKILL.md"
    target.unlink()
    manifest = load_manifest(manifest_path_for(target=tmp_path, profile="claude_code"))
    assert manifest is not None
    assert skill_files_present(target=tmp_path, manifest=manifest) == [
        ".claude/skills/striatum-recover/SKILL.md"
    ]


def test_manifest_records_striatum_version_for_staleness_checks(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    payload = load_manifest(manifest_path)
    assert payload is not None
    assert payload["striatum_version"] == STRIATUM_VERSION
    payload["striatum_version"] = "0.0.0-pretend-stale"
    manifest_path.write_text(
        json.dumps(payload, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    stale = load_manifest(manifest_path)
    assert stale is not None
    assert stale["striatum_version"] != STRIATUM_VERSION


def test_manifest_template_sha_can_be_checked_without_sqlite(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    payload = load_manifest(manifest_path)
    assert payload is not None
    payload["files"][0]["template_sha256"] = "00" * 32
    manifest_path.write_text(
        json.dumps(payload, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    changed = load_manifest(manifest_path)
    assert changed is not None
    mismatches = [
        entry["path"]
        for entry in changed["files"]
        if entry["template_sha256"] != bundled_template_sha256(entry["template"])
    ]
    assert mismatches == [payload["files"][0]["path"]]


def test_no_external_url_invariant(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    install_skills(target=tmp_path, profile="codex")
    install_skills(target=tmp_path, profile="gemini")
    install_skills(target=tmp_path, profile="generic")
    rendered = []
    for path in _claude_paths(tmp_path):
        rendered.append(path.read_text(encoding="utf-8"))
    for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp"):
        rendered.append(
            (tmp_path / f".codex/agents/striatum-{skill}.md").read_text(encoding="utf-8")
        )
    rendered.append(
        (tmp_path / "striatum-STRIATUM_GEMINI_GUIDE.md").read_text(encoding="utf-8")
    )
    rendered.append(
        (tmp_path / "striatum-STRIATUM_AGENT_GUIDE.md").read_text(encoding="utf-8")
    )
    for body in rendered:
        assert "http://" not in body
        assert "https://" not in body


def test_mcp_skill_body_contains_required_guidance(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    body = (tmp_path / ".claude/skills/striatum-mcp/SKILL.md").read_text(encoding="utf-8")
    for expected in (
        "tools/list",
        "tools/call",
        "confirm_write",
        "capability_missing",
        "token_revoked",
        "token_expired",
        "method_unknown",
        "mutations_disabled",
        "audit",
    ):
        assert expected in body


def test_manifest_excludes_itself(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    manifest = load_manifest(manifest_path)
    assert manifest is not None
    paths = {entry["path"] for entry in manifest["files"]}
    assert ".manifest.json" not in "".join(paths) or all(
        ".manifest.json" not in p for p in paths
    )


def test_bundled_template_sha_matches_manifest_after_install(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest = load_manifest(
        manifest_path_for(target=tmp_path, profile="claude_code")
    )
    assert manifest is not None
    for entry in manifest["files"]:
        assert (
            entry["template_sha256"]
            == bundled_template_sha256(entry["template"])
        )


def test_install_unknown_profile_raises(tmp_path: Path) -> None:
    from striatum.errors import InvalidTransitionError

    with pytest.raises(InvalidTransitionError):
        install_skills(target=tmp_path, profile="mystery")


# RFC 0015 step 3: codex + gemini profiles + --profile all -----------------


def _codex_paths(target: Path, namespace: str = "striatum-") -> list[Path]:
    return [
        target / ".codex" / "agents" / f"{namespace}{skill}.md"
        for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover", "mcp")
    ]


def test_install_codex_writes_files_and_manifest(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="codex")
    assert result["profile"] == "codex"
    assert {f["status"] for f in result["files"]} == {"written"}
    for path in _codex_paths(tmp_path):
        assert path.exists(), path
    manifest_path = Path(result["manifest_path"])
    manifest = load_manifest(manifest_path)
    assert manifest is not None
    assert manifest["profile"] == "codex"
    # The manifest sits next to its content under .codex/agents/.
    assert manifest_path.parent == tmp_path / ".codex" / "agents"
    # Codex shares Markdown bodies with claude_code; recorded
    # template_sha256 should equal claude_code's bundled SHA.
    for entry in manifest["files"]:
        assert entry["template"].startswith("claude_code/")


def test_install_codex_idempotent_byte_identical(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="codex")
    snapshot = {p: p.read_bytes() for p in _codex_paths(tmp_path)}
    second = install_skills(target=tmp_path, profile="codex")
    assert {f["status"] for f in second["files"]} == {"skipped_unchanged"}
    for path, data in snapshot.items():
        assert path.read_bytes() == data


def test_install_gemini_writes_single_guide(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="gemini")
    paths = [f["path"] for f in result["files"]]
    assert paths == ["striatum-STRIATUM_GEMINI_GUIDE.md"]
    guide = tmp_path / "striatum-STRIATUM_GEMINI_GUIDE.md"
    assert guide.exists()
    manifest = load_manifest(Path(result["manifest_path"]))
    assert manifest is not None
    assert manifest["profile"] == "gemini"
    # Distinct filename keeps `--profile all` collision-free with `generic`.
    assert "STRIATUM_GEMINI_GUIDE" in result["manifest_path"]


def test_install_gemini_idempotent_byte_identical(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="gemini")
    guide = tmp_path / "striatum-STRIATUM_GEMINI_GUIDE.md"
    snapshot = guide.read_bytes()
    second = install_skills(target=tmp_path, profile="gemini")
    assert {f["status"] for f in second["files"]} == {"skipped_unchanged"}
    assert guide.read_bytes() == snapshot


def _all_artifact_paths(target: Path) -> list[Path]:
    paths = list(_claude_paths(target))
    paths.extend(_codex_paths(target))
    paths.append(target / "striatum-STRIATUM_GEMINI_GUIDE.md")
    paths.append(target / "striatum-STRIATUM_AGENT_GUIDE.md")
    return paths


def test_install_profile_all_writes_every_profile(tmp_path: Path) -> None:
    import subprocess

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    proc = subprocess.run(
        [
            sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path),
            "skills", "install", "--profile", "all", "--json",
        ],
        env=env, capture_output=True, text=True, check=False,
    )
    assert proc.returncode == 0, proc.stderr
    data = json.loads(proc.stdout)["data"]
    assert data["profile"] == "all"
    assert len(data["results"]) == 4
    assert [r["profile"] for r in data["results"]] == [
        "claude_code", "codex", "gemini", "generic",
    ]
    for path in _all_artifact_paths(tmp_path):
        assert path.exists(), path


def test_install_profile_all_idempotent(tmp_path: Path) -> None:
    import subprocess

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    cmd = [
        sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path),
        "skills", "install", "--profile", "all", "--json",
    ]
    proc1 = subprocess.run(cmd, env=env, capture_output=True, text=True, check=False)
    assert proc1.returncode == 0
    snapshot = {p: p.read_bytes() for p in _all_artifact_paths(tmp_path)}
    proc2 = subprocess.run(cmd, env=env, capture_output=True, text=True, check=False)
    assert proc2.returncode == 0
    data = json.loads(proc2.stdout)["data"]
    statuses = {f["status"] for r in data["results"] for f in r["files"]}
    assert statuses == {"skipped_unchanged"}
    for path, data_bytes in snapshot.items():
        assert path.read_bytes() == data_bytes


def test_init_with_skills_all_writes_every_profile(tmp_path: Path) -> None:
    import subprocess

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    proc = subprocess.run(
        [
            sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path),
            "init", "--with-skills", "all", "--json",
        ],
        env=env, capture_output=True, text=True, check=False,
    )
    assert proc.returncode == 0, proc.stderr
    payload = json.loads(proc.stdout)
    assert payload["ok"] is True
    assert payload["data"]["skills"]["profile"] == "all"
    assert payload["data"]["state_store"] == "daemon_postgres"
    assert (tmp_path / ".striatum").is_dir()
    assert not (tmp_path / ".striatum/state.sqlite3").exists()
    for path in _all_artifact_paths(tmp_path):
        assert path.exists(), path


def test_skill_files_present_reports_missing_codex_profile_file(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="codex")
    target = tmp_path / ".codex" / "agents" / "striatum-recover.md"
    target.unlink()
    manifest = load_manifest(manifest_path_for(target=tmp_path, profile="codex"))
    assert manifest is not None
    assert skill_files_present(target=tmp_path, manifest=manifest) == [
        ".codex/agents/striatum-recover.md"
    ]


def test_skill_files_present_reports_missing_gemini_profile_file(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="gemini")
    (tmp_path / "striatum-STRIATUM_GEMINI_GUIDE.md").unlink()
    manifest = load_manifest(manifest_path_for(target=tmp_path, profile="gemini"))
    assert manifest is not None
    assert skill_files_present(target=tmp_path, manifest=manifest) == [
        "striatum-STRIATUM_GEMINI_GUIDE.md"
    ]
