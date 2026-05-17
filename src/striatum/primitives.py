"""Substrate-neutral primitives shared by Striatum modules."""

from __future__ import annotations

import hashlib
import json
import uuid
from datetime import UTC, datetime
from typing import Any, cast

from striatum.errors import InvalidTransitionError

JsonObject = dict[str, Any]


def utc_now() -> str:
    """Return an RFC3339 UTC timestamp."""
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    """Return an opaque stable-enough local id."""
    return f"{prefix}_{uuid.uuid4().hex}"


def json_dumps(value: object) -> str:
    """Serialize JSON deterministically for hashing and storage."""
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def json_loads(value: str) -> JsonObject:
    """Load a JSON object from a persisted text column."""
    loaded = json.loads(value)
    if not isinstance(loaded, dict):
        raise InvalidTransitionError("stored JSON value is not an object")
    return cast(JsonObject, loaded)


def sha256_bytes(payload: bytes) -> str:
    """Return a hex SHA-256 digest."""
    return hashlib.sha256(payload).hexdigest()
