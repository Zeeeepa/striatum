"""Daemon signing-key location helpers."""

from __future__ import annotations

import os
from pathlib import Path


ENV_SIGNING_KEY_PATH = "STRIATUM_DAEMON_SIGNING_KEY_PATH"
KEYRING_SERVICE = "striatum.daemon.signing_key"
DEFAULT_KEYRING_ACCOUNT = "default"


def fallback_key_path() -> Path:
    override = os.environ.get(ENV_SIGNING_KEY_PATH)
    if override:
        return Path(override).expanduser().resolve()
    state_home = Path(os.environ.get("XDG_STATE_HOME", Path.home() / ".local" / "state"))
    return state_home / "striatum" / "daemon" / "signing_key"


def keyring_key_loaded(*, account: str = DEFAULT_KEYRING_ACCOUNT) -> bool:
    try:
        import keyring
    except Exception:
        return False
    try:
        return bool(keyring.get_password(KEYRING_SERVICE, account))
    except Exception:
        return False


def fallback_key_loaded() -> bool:
    path = fallback_key_path()
    if not path.exists():
        return False
    mode = path.stat().st_mode & 0o777
    return (mode & 0o077) == 0


def key_loaded(*, account: str = DEFAULT_KEYRING_ACCOUNT) -> bool:
    return keyring_key_loaded(account=account) or fallback_key_loaded()
