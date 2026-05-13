"""Small git subprocess helpers for corpus export."""

from __future__ import annotations

import hashlib
import subprocess
from dataclasses import dataclass
from pathlib import Path

from striatum.errors import StriatumError


@dataclass(frozen=True)
class CommitRecord:
    sha: str
    author_date: str
    committer_date: str
    parents: tuple[str, ...]
    subject: str
    body: str
    changed_paths: tuple[str, ...]


def git(repo: Path, *args: str) -> str:
    """Run git and return stdout, mapping failures to domain errors."""
    try:
        result = subprocess.run(
            ["git", "-C", str(repo), *args],
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError as exc:
        raise StriatumError(f"git invocation failed: {exc}", exit_code=1) from exc
    if result.returncode != 0:
        message = result.stderr.strip() or result.stdout.strip() or "git command failed"
        raise StriatumError(message, exit_code=8)
    return result.stdout


def resolve_commit(repo: Path, ref: str) -> str:
    return git(repo, "rev-parse", "--verify", f"{ref}^{{commit}}").strip()


def head_commit(repo: Path) -> str:
    return git(repo, "rev-parse", "HEAD").strip()


def dirty_paths(repo: Path) -> set[str]:
    output = git(repo, "status", "--porcelain")
    paths: set[str] = set()
    for line in output.splitlines():
        if not line:
            continue
        raw = line[3:] if len(line) > 3 else line
        if " -> " in raw:
            raw = raw.rsplit(" -> ", 1)[1]
        paths.add(raw.strip())
    return paths


def is_dirty(repo: Path) -> bool:
    return bool(dirty_paths(repo))


def changed_paths(repo: Path, since_commit: str) -> set[str]:
    paths = set(git(repo, "diff", "--name-only", f"{since_commit}..HEAD").splitlines())
    paths.update(dirty_paths(repo))
    return {path for path in paths if path}


def file_sha256(repo: Path, rel_path: str) -> str:
    data = (repo / rel_path).read_bytes()
    return hashlib.sha256(data).hexdigest()


def last_touch(repo: Path, rel_path: str) -> tuple[str, str]:
    output = git(repo, "log", "-1", "--format=%H%x00%cI", "--", rel_path).strip()
    if output:
        commit, _, date = output.partition("\x00")
        return commit, normalize_git_timestamp(date)
    return head_commit(repo), normalize_git_timestamp(git(repo, "show", "-s", "--format=%cI", "HEAD").strip())


def commits_since(repo: Path, since_commit: str) -> list[CommitRecord]:
    fmt = "%H%x1f%aI%x1f%cI%x1f%P%x1f%s%x1f%b%x1e"
    output = git(repo, "log", f"{since_commit}..HEAD", f"--format={fmt}", "--reverse")
    records: list[CommitRecord] = []
    for raw in output.rstrip("\x1e\n").split("\x1e"):
        raw = raw.strip("\n")
        if not raw:
            continue
        parts = raw.split("\x1f")
        if len(parts) < 6:
            continue
        sha, author_date, committer_date, parents, subject, body = parts[:6]
        paths = tuple(
            path
            for path in git(repo, "diff-tree", "--no-commit-id", "--name-only", "-r", sha).splitlines()
            if path
        )
        records.append(
            CommitRecord(
                sha=sha,
                author_date=normalize_git_timestamp(author_date),
                committer_date=normalize_git_timestamp(committer_date),
                parents=tuple(parent for parent in parents.split(" ") if parent),
                subject=subject,
                body=body.strip(),
                changed_paths=paths,
            )
        )
    return records


def normalize_git_timestamp(value: str) -> str:
    # Git emits ISO-8601 offsets. RFC 0044 wants UTC-ish stable second
    # precision with a Z suffix; preserving the instant is unnecessary for
    # deterministic tests here, but dropping fractional seconds is required.
    from datetime import datetime, timezone

    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    return parsed.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
