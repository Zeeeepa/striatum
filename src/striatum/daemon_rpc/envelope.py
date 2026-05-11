"""Envelope-v1 codec for daemon RPC."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping

from striatum.db import json_dumps, json_loads
from striatum.errors import StriatumError

SUPPORTED_ENVELOPE_VERSION = 1
DEFAULT_FRAMING = "json"


class RpcError(StriatumError):
    """Stable daemon RPC refusal."""

    def __init__(self, code: str, message: str, *, exit_code: int = 10, details: Mapping[str, Any] | None = None) -> None:
        super().__init__(message, exit_code=exit_code)
        self.code = code
        self.details = dict(details or {})


@dataclass(frozen=True)
class RpcEnvelope:
    schema_version: int
    request_id: str
    method: str
    params: dict[str, Any]
    capability_token: str | None = None
    deadline_ms: int = 0

    @classmethod
    def from_mapping(cls, payload: Mapping[str, Any]) -> "RpcEnvelope":
        version = payload.get("schema_version")
        if version != SUPPORTED_ENVELOPE_VERSION:
            raise RpcError(
                "version_incompatible",
                "daemon RPC envelope version is not supported",
                details={"supported_envelope": [SUPPORTED_ENVELOPE_VERSION], "received": version},
            )
        request_id = payload.get("request_id")
        method = payload.get("method")
        params = payload.get("params")
        if not isinstance(request_id, str) or not request_id:
            raise RpcError("schema_invalid", "daemon RPC request_id must be a non-empty string")
        if not isinstance(method, str) or "." not in method:
            raise RpcError("schema_invalid", "daemon RPC method must be a dotted string")
        if not isinstance(params, dict):
            raise RpcError("schema_invalid", "daemon RPC params must be a JSON object")
        token = payload.get("capability_token")
        if token is not None and not isinstance(token, str):
            raise RpcError("schema_invalid", "daemon RPC capability_token must be a string when present")
        deadline = payload.get("deadline_ms", 0)
        if not isinstance(deadline, int) or deadline < 0:
            raise RpcError("schema_invalid", "daemon RPC deadline_ms must be a non-negative integer")
        return cls(
            schema_version=SUPPORTED_ENVELOPE_VERSION,
            request_id=request_id,
            method=method,
            params=dict(params),
            capability_token=token,
            deadline_ms=deadline,
        )

    @classmethod
    def decode(cls, body: bytes | str) -> "RpcEnvelope":
        try:
            payload = json_loads(body.decode("utf-8") if isinstance(body, bytes) else body)
        except Exception as exc:  # noqa: BLE001 - malformed transport body.
            raise RpcError("schema_invalid", "daemon RPC body is not valid JSON") from exc
        if not isinstance(payload, dict):
            raise RpcError("schema_invalid", "daemon RPC body must be a JSON object")
        return cls.from_mapping(payload)

    def to_mapping(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "schema_version": self.schema_version,
            "request_id": self.request_id,
            "method": self.method,
            "params": self.params,
            "deadline_ms": self.deadline_ms,
        }
        if self.capability_token is not None:
            payload["capability_token"] = self.capability_token
        return payload

    def encode(self) -> bytes:
        return json_dumps(self.to_mapping()).encode("utf-8")


@dataclass(frozen=True)
class RpcResponse:
    schema_version: int
    request_id: str
    ok: bool
    data: dict[str, Any]
    audit_id: str | None = None

    @classmethod
    def ok_response(cls, *, request_id: str, data: Mapping[str, Any], audit_id: str | None = None) -> "RpcResponse":
        return cls(
            schema_version=SUPPORTED_ENVELOPE_VERSION,
            request_id=request_id,
            ok=True,
            data=dict(data),
            audit_id=audit_id,
        )

    @classmethod
    def error_response(cls, *, request_id: str, error: RpcError, audit_id: str | None = None) -> "RpcResponse":
        return cls(
            schema_version=SUPPORTED_ENVELOPE_VERSION,
            request_id=request_id,
            ok=False,
            data={"code": error.code, "message": str(error), "details": error.details},
            audit_id=audit_id,
        )

    def to_mapping(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "schema_version": self.schema_version,
            "request_id": self.request_id,
            "ok": self.ok,
            "data": self.data,
        }
        if self.audit_id is not None:
            payload["audit_id"] = self.audit_id
        return payload

    def encode(self) -> bytes:
        return json_dumps(self.to_mapping()).encode("utf-8")
