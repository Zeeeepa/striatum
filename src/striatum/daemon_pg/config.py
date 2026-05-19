"""Configuration resolution for the daemon PostgreSQL substrate."""

from __future__ import annotations

import os
import sys
import tomllib
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

ENV_DAEMON_DB_URL = "STRIATUM_DAEMON_DB_URL"


@dataclass(frozen=True)
class PostgresConfig:
    url: str | None
    source: str | None
    config_path: Path

    @property
    def redacted_url(self) -> str | None:
        if self.url is None:
            return None
        return redact_url(self.url)


def default_config_path() -> Path:
    if sys.platform == "darwin":
        return Path.home() / "Library" / "Application Support" / "striatum" / "daemon.toml"
    return Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "striatum" / "daemon.toml"


def resolve_config(*, postgres_url: str | None = None) -> PostgresConfig:
    config_path = default_config_path()
    if postgres_url:
        return PostgresConfig(postgres_url, "--postgres-url", config_path)
    env_url = os.environ.get(ENV_DAEMON_DB_URL)
    if env_url:
        return PostgresConfig(env_url, ENV_DAEMON_DB_URL, config_path)
    file_url = _read_config_url(config_path)
    if file_url:
        return PostgresConfig(file_url, str(config_path), config_path)
    return PostgresConfig(None, None, config_path)


def _read_config_url(path: Path) -> str | None:
    try:
        data = tomllib.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return None
    raw = data.get("postgres_url")
    if isinstance(raw, str) and raw.strip():
        return raw.strip()
    daemon = data.get("daemon")
    if isinstance(daemon, dict):
        nested = daemon.get("postgres_url")
        if isinstance(nested, str) and nested.strip():
            return nested.strip()
    return None


def redact_url(url: str) -> str:
    parts = urlsplit(url)
    netloc = parts.netloc
    has_credentials = "@" in netloc
    if has_credentials:
        userinfo, host = netloc.rsplit("@", 1)
        if ":" in userinfo:
            username, _password = userinfo.split(":", 1)
            netloc = f"{username}:<redacted>@{host}"
        else:
            netloc = f"{userinfo}@{host}"
    query_pairs = []
    sensitive_query = False
    for key, value in parse_qsl(parts.query, keep_blank_values=True):
        if key.lower() in {"password", "pass", "token", "sslpassword"}:
            query_pairs.append((key, "<redacted>"))
            sensitive_query = True
        else:
            query_pairs.append((key, value))
    # urlunsplit collapses the libpq peer-auth `postgresql:///db` triple slash
    # to a single slash because urlsplit reports an empty netloc. When there
    # is nothing to redact, returning the original input preserves the
    # operator-recognizable URL shape (GH #22).
    if not has_credentials and not sensitive_query:
        return url
    return urlunsplit((parts.scheme, netloc, parts.path, urlencode(query_pairs), parts.fragment))


def onboarding_hints() -> list[dict[str, str]]:
    return [
        {"platform": "macos-homebrew", "hint": "brew install postgresql@16 && brew services start postgresql@16"},
        {"platform": "debian-ubuntu", "hint": "sudo apt install postgresql && sudo systemctl enable --now postgresql"},
        {"platform": "arch", "hint": "sudo pacman -S postgresql; sudo -iu postgres initdb -D /var/lib/postgres/data; sudo systemctl enable --now postgresql"},
        {"platform": "freebsd-pkg", "hint": "sudo pkg install postgresql16-server postgresql16-client"},
        {"platform": "wsl", "hint": "install PostgreSQL inside the Linux distribution and use Linux repo paths"},
    ]
