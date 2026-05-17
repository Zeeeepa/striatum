"""Pure HTTP/security helpers for the local Striatum service."""

from __future__ import annotations

import hashlib
import hmac
from urllib.parse import urlsplit

OriginTuple = tuple[str, str, int]

LOOPBACK_HOSTS = frozenset({"127.0.0.1", "localhost", "::1"})
HTTP_TOKEN_CHARS = frozenset(
    "!#$%&'*+-.^_`|~0123456789"
    "abcdefghijklmnopqrstuvwxyz"
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)


def tokens_match(provided: str, expected: str) -> bool:
    """Constant-time token comparison that masks length differences."""
    p = provided.encode("utf-8")
    e = expected.encode("utf-8")
    target = max(len(p), len(e), 64)
    p_padded = p.ljust(target, b"\x00")
    e_padded = e.ljust(target, b"\x00")
    return hmac.compare_digest(p_padded, e_padded) and len(p) == len(e)


def argv_value(argv: list[str], flag: str) -> str | None:
    """Return the value for ``--flag`` in an argv list, or ``None``."""
    for index, token in enumerate(argv):
        if token == flag and index + 1 < len(argv):
            return argv[index + 1]
        if token.startswith(flag + "="):
            return token[len(flag) + 1 :]
    return None


def is_json_content_type(ctype: str) -> bool:
    """Return whether a Content-Type value is exactly JSON plus valid params."""
    if not ctype or "," in ctype or "\r" in ctype or "\n" in ctype:
        return False
    parts = ctype.split(";")
    base = parts[0].strip().lower()
    if base != "application/json":
        return False
    for raw_param in parts[1:]:
        param = raw_param.strip()
        if not param:
            return False
        name, separator, value = param.partition("=")
        if not separator:
            return False
        if not _is_http_token(name.strip()):
            return False
        if not _is_content_type_param_value(value.strip()):
            return False
    return True


def _is_http_token(value: str) -> bool:
    return bool(value) and all(ch in HTTP_TOKEN_CHARS for ch in value)


def _is_content_type_param_value(value: str) -> bool:
    if not value:
        return False
    if value.startswith('"'):
        if len(value) < 2 or not value.endswith('"'):
            return False
        inner = value[1:-1]
        return "\r" not in inner and "\n" not in inner
    return _is_http_token(value)


def _loopback_aliases(host: str) -> set[str]:
    normalized = host.strip().lower()
    if normalized == "localhost":
        return {"localhost", "127.0.0.1", "::1"}
    if normalized == "127.0.0.1":
        return {"127.0.0.1", "localhost"}
    if normalized == "::1":
        return {"::1", "localhost"}
    return {normalized}


def allowed_origins_for_bind(host: str, port: int) -> set[OriginTuple]:
    return {("http", alias, port) for alias in _loopback_aliases(host)}


def parse_host_origin(host_header: str) -> OriginTuple | None:
    """Parse a request Host header into the service's HTTP origin tuple."""
    value = host_header.strip()
    if not value or "," in value or "://" in value or "@" in value:
        return None
    try:
        parsed = urlsplit("//" + value)
        port = parsed.port
    except ValueError:
        return None
    if parsed.hostname is None or port is None:
        return None
    return ("http", parsed.hostname.lower(), int(port))


def parse_header_origin(origin_or_referer: str) -> OriginTuple | None:
    """Return the origin tuple of an Origin or Referer header."""
    if not origin_or_referer:
        return None
    value = origin_or_referer.strip()
    if value == "null" or "://" not in value:
        return None
    try:
        parsed = urlsplit(value)
    except ValueError:
        return None
    if parsed.scheme != "http" or not parsed.netloc:
        return None
    try:
        port = parsed.port
    except ValueError:
        return None
    if parsed.hostname is None:
        return None
    return ("http", parsed.hostname.lower(), int(port) if port is not None else 80)


def make_web_context_token(secret: bytes, *, run_id: str, job_id: str, session_id: str) -> str:
    """Mint a process-local HMAC token for web override-verdict actions."""
    payload = "\x1f".join(["override_verdict", run_id, job_id, session_id]).encode("utf-8")
    return hashlib.blake2b(payload, key=secret, digest_size=16).hexdigest()


def verify_web_context_token(
    secret: bytes,
    *,
    token: str,
    run_id: str,
    job_id: str,
    session_id: str,
) -> bool:
    expected = make_web_context_token(
        secret,
        run_id=run_id,
        job_id=job_id,
        session_id=session_id,
    )
    return hmac.compare_digest(expected, token)


__all__ = [
    "LOOPBACK_HOSTS",
    "OriginTuple",
    "allowed_origins_for_bind",
    "argv_value",
    "is_json_content_type",
    "make_web_context_token",
    "parse_header_origin",
    "parse_host_origin",
    "tokens_match",
    "verify_web_context_token",
]
