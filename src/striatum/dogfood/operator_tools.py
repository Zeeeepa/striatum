"""Dogfood operator composite compatibility facade."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.errors import InvalidTransitionError
from striatum.primitives import JsonObject


class PublishOnBehalfDenied(InvalidTransitionError):
    """Compatibility exception for callers that prefer exception denials."""

    def __init__(self, denial_code: str, message: str) -> None:
        super().__init__(message)
        self.denial_code = denial_code


def publish_on_behalf(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_operator_tools().publish_on_behalf(*args, **kwargs))


def surgical_recovery(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_operator_tools().surgical_recovery(*args, **kwargs))


def __getattr__(name: str) -> Any:
    if name.startswith("__"):
        raise AttributeError(name)
    return getattr(_legacy_operator_tools(), name)


def _legacy_operator_tools() -> Any:
    return import_module("striatum.legacy_sqlite.dogfood_operator_tools")


__all__ = ["PublishOnBehalfDenied", "publish_on_behalf", "surgical_recovery"]
