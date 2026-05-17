from __future__ import annotations

import json
import os
from pathlib import Path

from striatum.web import chat_session


def test_chat_session_path_preserves_scratch_layout_and_rejects_unsafe_ids(tmp_path: Path) -> None:
    assert chat_session.chat_session_path(tmp_path, "abc123") == (
        tmp_path / ".striatum" / "scratch" / "chat-abc123" / "transcript.jsonl"
    )
    assert chat_session.chat_session_path(tmp_path, "") is None
    assert chat_session.chat_session_path(tmp_path, "a/b") is None
    assert chat_session.chat_session_path(tmp_path, "..") is None
    assert chat_session.chat_session_path(tmp_path, "a..b") is None


def test_list_chat_sessions_counts_nonblank_transcript_lines(tmp_path: Path) -> None:
    scratch = tmp_path / ".striatum" / "scratch"
    (scratch / "chat-alpha").mkdir(parents=True)
    (scratch / "chat-alpha" / "transcript.jsonl").write_text("{}\n\n{}\n", encoding="utf-8")
    (scratch / "chat-empty").mkdir()
    (scratch / "not-chat").mkdir()
    (scratch / "not-chat" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")
    os.utime(scratch / "chat-alpha", (1_700_000_000, 1_700_000_000))

    assert chat_session.list_chat_sessions(tmp_path) == [
        {
            "id": "alpha",
            "message_count": 2,
            "started_at": chat_session.format_ts(1_700_000_000),
        }
    ]


def test_read_display_messages_projects_transcript_for_template(tmp_path: Path) -> None:
    path = tmp_path / "transcript.jsonl"
    entries = [
        {"role": "assistant", "content": "partial", "streaming": True},
        {"role": "assistant", "content": "**done**", "created_at": "t1"},
        {"role": "user", "content": "<hello>\nthere", "created_at": "t2"},
        {
            "role": "tool_use",
            "tool_name": "read_file",
            "tool_input": {"path": "README.md"},
            "created_at": "t3",
        },
        {
            "role": "tool_result",
            "tool_name": "read_file",
            "result": "ok",
            "created_at": "t4",
        },
        {
            "role": "tool_confirmation",
            "tool_name": "generate_workflow_write",
            "tool_input": {"confirm_write": True},
            "tool_use_id": "call_1",
            "confirmation_token": "tok",
            "state": "pending",
            "spec_hash": "sha256:x",
            "created_at": "t5",
        },
        {"role": "system", "content": "system note", "created_at": "t6"},
    ]
    path.write_text(
        "\n".join(json.dumps(entry) for entry in entries) + "\n{bad json\n",
        encoding="utf-8",
    )

    messages = chat_session.read_display_messages(
        path,
        markdown_render=lambda text: f"<md>{text}</md>",
        html_escape=lambda text: text.replace("<", "&lt;").replace(">", "&gt;"),
    )

    assert [message["role"] for message in messages] == [
        "assistant",
        "user",
        "tool_use",
        "tool_result",
        "tool_confirmation",
        "system",
    ]
    assert messages[0]["rendered"] == "<md>**done**</md>"
    assert messages[1]["rendered"] == "<p>&lt;hello&gt;<br>there</p>"
    assert messages[2]["tool_name"] == "read_file"
    assert messages[4]["confirmation_token"] == "tok"
    assert messages[5]["rendered"] == "<md>system note</md>"


def test_queue_workflow_write_confirmation_validates_and_appends_pending(
    tmp_path: Path,
) -> None:
    path = tmp_path / "transcript.jsonl"
    assert chat_session.queue_workflow_write_confirmation(
        path=path,
        session_id="sess",
        tool_id="call_1",
        tool_args={"confirm_write": True, "spec": {"workflow_id": "wf"}},
        allow_mutations=False,
        token_factory=lambda: "tok",
    ).startswith("[error] mutations_disabled")
    assert not path.exists()

    assert chat_session.queue_workflow_write_confirmation(
        path=path,
        session_id="sess",
        tool_id="call_1",
        tool_args={"spec": {"workflow_id": "wf"}},
        allow_mutations=True,
        token_factory=lambda: "tok",
    ).startswith("[error] confirm_write_missing")
    assert not path.exists()

    assert chat_session.queue_workflow_write_confirmation(
        path=path,
        session_id="sess",
        tool_id="call_1",
        tool_args={"confirm_write": True},
        allow_mutations=True,
        token_factory=lambda: "tok",
    ) == "[error] missing spec object"
    assert not path.exists()

    spec = {"workflow_id": "wf"}
    assert chat_session.queue_workflow_write_confirmation(
        path=path,
        session_id="sess",
        tool_id="call_1",
        tool_args={"confirm_write": True, "spec": spec},
        allow_mutations=True,
        token_factory=lambda: "tok",
    ).startswith("[pending]")

    entry = json.loads(path.read_text(encoding="utf-8"))
    assert entry["role"] == "tool_confirmation"
    assert entry["confirmation_token"] == "tok"
    assert entry["spec_hash"] == chat_session.stable_json_hash(spec)


def test_find_pending_tool_confirmation_returns_latest_pending_until_used(tmp_path: Path) -> None:
    path = tmp_path / "transcript.jsonl"
    chat_session.append_jsonl(
        path,
        {"role": "tool_confirmation", "tool_use_id": "call_1", "state": "pending", "confirmation_token": "old"},
    )
    chat_session.append_jsonl(
        path,
        {"role": "tool_confirmation", "tool_use_id": "call_1", "state": "pending", "confirmation_token": "new"},
    )
    chat_session.append_jsonl(
        path,
        {"role": "tool_confirmation", "tool_use_id": "call_2", "state": "pending", "confirmation_token": "other"},
    )

    pending = chat_session.find_pending_tool_confirmation(path=path, tool_id="call_1")
    assert pending is not None
    assert pending["confirmation_token"] == "new"

    chat_session.append_jsonl(
        path,
        {"role": "tool_confirmation", "tool_use_id": "call_1", "state": "used", "confirmation_token": "<used>"},
    )

    assert chat_session.find_pending_tool_confirmation(path=path, tool_id="call_1") is None
    other = chat_session.find_pending_tool_confirmation(path=path, tool_id="call_2")
    assert other is not None
    assert other["confirmation_token"] == "other"
