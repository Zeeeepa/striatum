from __future__ import annotations

from striatum.service_http import make_web_context_token
from striatum.service_request_security import (
    authenticate_request,
    has_valid_bearer,
    requires_same_origin,
    verify_override_verdict_context,
    verify_same_origin_mutation,
)


def test_authenticate_request_allows_when_no_token_configured() -> None:
    decision = authenticate_request({}, token=None)

    assert decision.ok is True


def test_authenticate_request_rejects_missing_or_malformed_bearer() -> None:
    for headers in ({}, {"Authorization": "Basic nope"}):
        decision = authenticate_request(headers, token="secret")

        assert decision.ok is False
        assert decision.status == 401
        assert decision.error == {
            "code": 401,
            "message": "missing or invalid Authorization header",
        }


def test_authenticate_request_checks_bearer_token() -> None:
    wrong = authenticate_request({"Authorization": "Bearer wrong"}, token="secret")
    right = authenticate_request({"Authorization": "Bearer secret"}, token="secret")

    assert wrong.ok is False
    assert wrong.error == {"code": 401, "message": "invalid token"}
    assert right.ok is True
    assert has_valid_bearer({"Authorization": "Bearer secret"}, token="secret") is True
    assert has_valid_bearer({"Authorization": "Bearer wrong"}, token="secret") is False


def test_requires_same_origin_policy() -> None:
    assert requires_same_origin(
        "/v1/invoke",
        origin_check_enabled=False,
        web_enabled=True,
    ) is False
    assert requires_same_origin(
        "/v1/invoke",
        origin_check_enabled=True,
        web_enabled=False,
    ) is True
    assert requires_same_origin(
        "/chat/new",
        origin_check_enabled=True,
        web_enabled=True,
    ) is True
    assert requires_same_origin(
        "/chat/new",
        origin_check_enabled=True,
        web_enabled=False,
    ) is False


def test_verify_same_origin_mutation_allows_valid_bearer_without_origin() -> None:
    decision = verify_same_origin_mutation(
        {"Authorization": "Bearer secret"},
        token="secret",
        allowed_origins={("http", "127.0.0.1", 1234)},
    )

    assert decision.ok is True


def test_verify_same_origin_mutation_rejects_bad_host() -> None:
    decision = verify_same_origin_mutation(
        {"Host": "evil.example:1234", "Origin": "http://evil.example:1234"},
        token=None,
        allowed_origins={("http", "127.0.0.1", 1234)},
    )

    assert decision.ok is False
    assert decision.status == 403
    assert decision.error == {"code": 403, "message": "Host header origin refused"}


def test_verify_same_origin_mutation_checks_origin_and_referer() -> None:
    allowed = {("http", "127.0.0.1", 1234)}

    assert verify_same_origin_mutation(
        {"Host": "127.0.0.1:1234", "Origin": "http://127.0.0.1:1234"},
        token=None,
        allowed_origins=allowed,
    ).ok is True
    assert verify_same_origin_mutation(
        {"Host": "127.0.0.1:1234", "Referer": "http://127.0.0.1:1234/run/r"},
        token=None,
        allowed_origins=allowed,
    ).ok is True

    bad_origin = verify_same_origin_mutation(
        {"Host": "127.0.0.1:1234", "Origin": "http://evil.example"},
        token=None,
        allowed_origins=allowed,
    )
    bad_referer = verify_same_origin_mutation(
        {"Host": "127.0.0.1:1234", "Referer": "http://evil.example/page"},
        token=None,
        allowed_origins=allowed,
    )
    missing = verify_same_origin_mutation(
        {"Host": "127.0.0.1:1234"},
        token=None,
        allowed_origins=allowed,
    )

    assert bad_origin.error == {"code": 403, "message": "cross-origin request refused"}
    assert bad_referer.error == {"code": 403, "message": "cross-origin request refused"}
    assert missing.error == {"code": 403, "message": "Origin or Referer required"}


def test_verify_override_verdict_context_rejects_missing_or_malformed_context() -> None:
    missing = verify_override_verdict_context(
        ["override-verdict"],
        {},
        web_context_secret=b"secret",
    )
    malformed = verify_override_verdict_context(
        ["override-verdict"],
        {"web_context": {"kind": "override_verdict"}},
        web_context_secret=b"secret",
    )

    assert missing.error == {"code": 403, "message": "override-verdict requires web_context"}
    assert malformed.error == {
        "code": 403,
        "message": "web_context fields missing or malformed",
    }


def test_verify_override_verdict_context_checks_argv_and_token() -> None:
    secret = b"secret"
    valid_token = make_web_context_token(
        secret,
        run_id="run-1",
        job_id="job-1",
        session_id="session-1",
    )
    argv = [
        "override-verdict",
        "--session-id",
        "session-1",
        "--job-id",
        "job-1",
    ]
    body = {
        "web_context": {
            "kind": "override_verdict",
            "run_id": "run-1",
            "job_id": "job-1",
            "session_id": "session-1",
            "token": valid_token,
        },
    }

    assert verify_override_verdict_context(
        argv,
        body,
        web_context_secret=secret,
    ).ok is True

    mismatch = verify_override_verdict_context(
        ["override-verdict", "--session-id", "session-1", "--job-id", "other"],
        body,
        web_context_secret=secret,
    )
    forged = verify_override_verdict_context(
        argv,
        {
            "web_context": {
                **body["web_context"],
                "token": "deadbeef" * 4,
            },
        },
        web_context_secret=secret,
    )

    assert mismatch.error == {"code": 403, "message": "argv does not match web_context"}
    assert forged.error == {"code": 403, "message": "invalid web_context token"}
