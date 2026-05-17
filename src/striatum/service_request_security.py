"""Request security decisions for the local service."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping

from striatum.service_http import (
    OriginTuple,
    argv_value,
    parse_header_origin,
    parse_host_origin,
    tokens_match,
    verify_web_context_token,
)

JsonObject = dict[str, Any]

__all__ = [
    "SecurityDecision",
    "authenticate_request",
    "has_valid_bearer",
    "requires_same_origin",
    "verify_override_verdict_context",
    "verify_same_origin_mutation",
]


@dataclass(frozen=True)
class SecurityDecision:
    ok: bool
    status: int = 200
    error: JsonObject | None = None


def _error(status: int, message: str) -> SecurityDecision:
    return SecurityDecision(
        False,
        status,
        {"code": status, "message": message},
    )


def _header(headers: Mapping[str, str], name: str) -> str:
    value = headers.get(name)
    if value is not None:
        return value
    lower_name = name.lower()
    for key, raw in headers.items():
        if key.lower() == lower_name:
            return raw
    return ""


def authenticate_request(
    headers: Mapping[str, str],
    *,
    token: str | None,
) -> SecurityDecision:
    if token is None:
        return SecurityDecision(True)
    header = _header(headers, "Authorization")
    prefix = "Bearer "
    if not header.startswith(prefix):
        return _error(401, "missing or invalid Authorization header")
    provided = header[len(prefix):]
    if not tokens_match(provided, token):
        return _error(401, "invalid token")
    return SecurityDecision(True)


def has_valid_bearer(headers: Mapping[str, str], *, token: str | None) -> bool:
    if token is None:
        return False
    header = _header(headers, "Authorization")
    prefix = "Bearer "
    if not header.startswith(prefix):
        return False
    return tokens_match(header[len(prefix):], token)


def requires_same_origin(
    path: str,
    *,
    origin_check_enabled: bool,
    web_enabled: bool,
) -> bool:
    if not origin_check_enabled:
        return False
    return path == "/v1/invoke" or web_enabled


def verify_same_origin_mutation(
    headers: Mapping[str, str],
    *,
    token: str | None,
    allowed_origins: set[OriginTuple],
) -> SecurityDecision:
    """Reject cross-origin browser POSTs to mutation surfaces."""
    if has_valid_bearer(headers, token=token):
        return SecurityDecision(True)
    host_origin = parse_host_origin(_header(headers, "Host"))
    if host_origin not in allowed_origins:
        return _error(403, "Host header origin refused")
    origin = _header(headers, "Origin")
    if origin:
        origin_value = parse_header_origin(origin)
        if origin_value not in allowed_origins:
            return _error(403, "cross-origin request refused")
        return SecurityDecision(True)
    referer = _header(headers, "Referer")
    if referer:
        referer_value = parse_header_origin(referer)
        if referer_value not in allowed_origins:
            return _error(403, "cross-origin request refused")
        return SecurityDecision(True)
    return _error(403, "Origin or Referer required")


def verify_override_verdict_context(
    argv: list[str],
    body: Mapping[str, Any],
    *,
    web_context_secret: bytes,
) -> SecurityDecision:
    ctx = body.get("web_context")
    if not isinstance(ctx, dict):
        return _error(403, "override-verdict requires web_context")
    kind = ctx.get("kind")
    run_id = ctx.get("run_id")
    job_id = ctx.get("job_id")
    session_id = ctx.get("session_id")
    token = ctx.get("token")
    if (
        kind != "override_verdict"
        or not isinstance(run_id, str) or not run_id
        or not isinstance(job_id, str) or not job_id
        or not isinstance(session_id, str) or not session_id
        or not isinstance(token, str) or not token
    ):
        return _error(403, "web_context fields missing or malformed")
    argv_job = argv_value(argv, "--job-id")
    argv_session = argv_value(argv, "--session-id")
    if argv_job != job_id or argv_session != session_id:
        return _error(403, "argv does not match web_context")
    if not verify_web_context_token(
        web_context_secret,
        token=token,
        run_id=run_id,
        job_id=job_id,
        session_id=session_id,
    ):
        return _error(403, "invalid web_context token")
    return SecurityDecision(True)
