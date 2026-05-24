"""Shared parity helpers for RFC 0048 PG vs retired local-state handler tests.

Per RFC 0048 V1.5 #1 (claude ergonomics_dx HIGH): the substrate-flip
acceptance criterion is byte-identical state between the PG-backed
handler and the retired local state CLI helper for the same input fixture. This
module provides the assertion helper and the shared parity decorator
the per-handler tests use.

Usage in a test file:

    from tests.daemon_pg.handlers._parity import assert_payload_parity

    def test_my_handler_parity(pg_ctx, sqlite_conn):
        seed_pg(pg_ctx, fixture)
        seed_sqlite(sqlite_conn, fixture)
        pg_out = pg_handler.handle(pg_ctx, params)
        sqlite_out = sqlite_handler(sqlite_conn, **params)
        assert_payload_parity(pg_out, sqlite_out,
                              ignore_keys=("acquired_at", "expires_at"))
"""

from __future__ import annotations

from typing import Any, Iterable


_NORMALIZE_NONE = object()


def assert_payload_parity(
    pg_result: Any,
    sqlite_result: Any,
    *,
    ignore_keys: Iterable[str] = (),
    path: str = "",
) -> None:
    """Assert ``pg_result`` and ``sqlite_result`` are byte-equivalent.

    Recurses into dicts and lists. Keys listed in ``ignore_keys`` are
    stripped before comparison at every dict level (not just the root) —
    timestamps and UUIDs that legitimately differ across substrates fit
    here. On mismatch, raises ``AssertionError`` with a human-readable
    per-key diff listing every differing path.

    Parameters
    ----------
    pg_result
        Payload returned by the PG-backed handler.
    sqlite_result
        Payload returned by the retired local-state handler.
    ignore_keys
        Dict keys to drop from both sides before comparison.
    path
        Internal — used for recursive error messages.
    """
    ignore = set(ignore_keys)
    pg_clean = _strip(pg_result, ignore)
    sqlite_clean = _strip(sqlite_result, ignore)
    diffs: list[str] = []
    _diff(pg_clean, sqlite_clean, path or "<root>", diffs)
    if diffs:
        raise AssertionError(
            "PG vs retired local-state handler payload parity diff:\n"
            + "\n".join(f"  - {line}" for line in diffs)
        )


def _strip(value: Any, ignore: set[str]) -> Any:
    if isinstance(value, dict):
        return {k: _strip(v, ignore) for k, v in value.items() if k not in ignore}
    if isinstance(value, list):
        return [_strip(item, ignore) for item in value]
    if isinstance(value, tuple):
        return tuple(_strip(item, ignore) for item in value)
    return value


def _diff(left: Any, right: Any, path: str, diffs: list[str]) -> None:
    if type(left) is not type(right):
        diffs.append(f"{path}: type mismatch ({type(left).__name__} vs {type(right).__name__})")
        return
    if isinstance(left, dict):
        left_keys = set(left)
        right_keys = set(right)
        for missing in sorted(left_keys - right_keys):
            diffs.append(f"{path}.{missing}: present in PG, missing in retired local-state")
        for extra in sorted(right_keys - left_keys):
            diffs.append(f"{path}.{extra}: missing in PG, present in retired local-state")
        for key in sorted(left_keys & right_keys):
            _diff(left[key], right[key], f"{path}.{key}", diffs)
        return
    if isinstance(left, list):
        if len(left) != len(right):
            diffs.append(f"{path}: list length differs ({len(left)} vs {len(right)})")
            return
        for index, (left_item, right_item) in enumerate(zip(left, right)):
            _diff(left_item, right_item, f"{path}[{index}]", diffs)
        return
    if left != right:
        diffs.append(f"{path}: {left!r} != {right!r}")


__all__ = ["assert_payload_parity"]
