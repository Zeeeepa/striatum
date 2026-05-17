"""PG-backed ``work.send_message`` handler."""

from __future__ import annotations

import json
from collections.abc import Mapping
from typing import Any, cast

from striatum.daemon_pg.handlers.context import RepoHandlerContext, _jsonb, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError


@register_pg_handler("work.send_message")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    kind = str(params["kind"])
    body = _body_from_params(params)

    with transaction(ctx):
        session = ctx.row_by_id("sessions", "session_id", session_id, for_update=True)
        message_id = ctx.new_id("msg")
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.queue_messages (
                  repository_id, message_id, run_id, kind, state, payload_json,
                  created_at, updated_at
                )
                VALUES (%s, %s, %s, 'agent_message', 'completed', %s, %s, %s)
                """,
                (
                    ctx.repository_id,
                    message_id,
                    session["run_id"],
                    _jsonb({"kind": kind, "body": body}),
                    now,
                    now,
                ),
            )
        ctx.append_event(
            run_id=str(session["run_id"]),
            event_type="message.sent",
            actor_session_id=session_id,
            message_id=message_id,
            payload={"kind": kind},
        )
        return {"message_id": message_id}


def _body_from_params(params: Mapping[str, Any]) -> dict[str, Any]:
    if "body" in params:
        body = params["body"]
    else:
        body_json = params.get("body_json", "{}")
        body = json.loads(str(body_json))
    if not isinstance(body, dict):
        raise InvalidTransitionError("message body must be a JSON object")
    return cast(dict[str, Any], body)
