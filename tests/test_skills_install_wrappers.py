from __future__ import annotations

import os
from pathlib import Path

from striatum.skills import install as install_skills


def test_skills_install_copies_supervised_wrappers_to_target_repo(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="codex")

    wrapper = tmp_path / ".striatum" / "bin" / "codex-supervised-wrapper.sh"
    assert result["profile"] == "codex"
    assert wrapper.exists()
    assert os.access(wrapper, os.X_OK)


def test_skills_install_dry_run_does_not_create_wrapper_scratch(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="codex", dry_run=True)

    assert not (tmp_path / ".striatum").exists()
