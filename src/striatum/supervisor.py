"""Supervisor constants and legacy compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

SUPERVISOR_ACTIVE_STATES = ("starting", "attached", "detached")

__all__ = [
    "SUPERVISOR_ACTIVE_STATES",
    "deliver_packet_to_attached_supervisor",
    "mark_supervisor_lost_for_lease",
    "supervise_list",
    "supervise_send",
    "supervise_start",
    "supervise_status",
    "supervise_stop",
]


def supervise_start(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_supervisor().supervise_start(*args, **kwargs))


def supervise_send(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_supervisor().supervise_send(*args, **kwargs))


def supervise_stop(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_supervisor().supervise_stop(*args, **kwargs))


def supervise_status(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_supervisor().supervise_status(*args, **kwargs))


def supervise_list(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_supervisor().supervise_list(*args, **kwargs))


def mark_supervisor_lost_for_lease(*args: Any, **kwargs: Any) -> None:
    _legacy_supervisor().mark_supervisor_lost_for_lease(*args, **kwargs)


def deliver_packet_to_attached_supervisor(
    *args: Any, **kwargs: Any
) -> JsonObject | None:
    return cast(
        JsonObject | None,
        _legacy_supervisor().deliver_packet_to_attached_supervisor(*args, **kwargs),
    )


def _legacy_supervisor() -> Any:
    return import_module("striatum.legacy_sqlite.supervisor")
