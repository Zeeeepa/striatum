"""Capability-token hash compatibility helpers."""

from __future__ import annotations

import hashlib
import hmac


def hash_token(*, secret: str, salt: str) -> str:
    """Return the V1 daemon capability-token hash."""

    return hmac.new(salt.encode("utf-8"), secret.encode("utf-8"), hashlib.sha256).hexdigest()
