from __future__ import annotations

import io
import json
import os
import sqlite3
import subprocess
import sys
from copy import deepcopy
from pathlib import Path
from typing import Any, cast

from striatum.api import invoke
from striatum.mcp import LocalRpcServer, serve_stdio

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"
DOCS_REVIEW_WORKFLOW = ROOT / "examples" / "docs-review-flow" / "workflow.json"


JsonDict = dict[str, Any]


def run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}")
    if result.stdout.strip() == "":
        return {}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def run_cli_text(repo: Path, *args: str, check: bool = True) -> str:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}")
    return result.stdout


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def api_data(payload: JsonDict) -> JsonDict:
    assert payload["ok"] is True
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def rpc_result(server: LocalRpcServer, method: str, params: JsonDict | None = None) -> JsonDict:
    request: JsonDict = {"jsonrpc": "2.0", "id": 1, "method": method}
    if params is not None:
        request["params"] = params
    response = server.handle_line(json.dumps(request))
    assert response is not None
    assert "error" not in response
    result = response["result"]
    assert isinstance(result, dict)
    return cast(JsonDict, result)


def init_repo(repo: Path) -> None:
    run_cli(repo, "init")


def prepare_started_run(repo: Path, workflow_path: Path = WORKFLOW) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    before = run_cli(repo, "claim-next", "--session-id", "missing", check=False)
    assert before["returncode"] == 3
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register(repo: Path, run_id: str, role: str, lane: str) -> str:
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            role,
            "--lane",
            lane,
            "--capability",
            "review",
        )
    )
    return str(payload["session_id"])


def claim(repo: Path, session_id: str) -> JsonDict:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return cast(JsonDict, packet)


def packet_ids(packet: JsonDict) -> tuple[str, str, str]:
    job = packet["job"]
    lease = packet["lease"]
    assert isinstance(job, dict)
    assert isinstance(lease, dict)
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def write_artifact(repo: Path, path: str, text: str = "artifact\n") -> None:
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def artifact_count(repo: Path, job_id: str) -> int:
    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute("SELECT COUNT(*) FROM artifacts WHERE job_id = ?", (job_id,)).fetchone()
    finally:
        conn.close()
    assert row is not None
    return int(row[0])


def temporary_workflow(tmp_path: Path, workflow: JsonDict) -> Path:
    path = tmp_path / "workflow.json"
    path.write_text(json.dumps(workflow), encoding="utf-8")
    return path


def example_workflow() -> JsonDict:
    loaded = cast(JsonDict, json.loads(WORKFLOW.read_text(encoding="utf-8")))
    assert isinstance(loaded, dict)
    return loaded


def complete_claimed_job(
    repo: Path,
    session_id: str,
    packet: JsonDict,
    *,
    logical_name: str,
    kind: str,
    path: str,
) -> None:
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(repo, "ack", "--session-id", session_id, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(repo, path)
    run_cli(
        repo,
        "publish-artifact",
        "--session-id",
        session_id,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        kind,
        "--logical-name",
        logical_name,
        "--path",
        path,
    )
    run_cli(repo, "complete", "--session-id", session_id, "--job-id", job_id, "--lease-id", lease_id)


def verdict_claimed_review(
    repo: Path,
    session_id: str,
    packet: JsonDict,
    *,
    verdict: str,
    logical_name: str = "review",
    kind: str = "finding",
    path: str,
    rationale: str | None = None,
) -> JsonDict:
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(repo, "ack", "--session-id", session_id, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(repo, path, text=f"{verdict}\n")
    artifact = data(
        run_cli(
            repo,
            "publish-artifact",
            "--session-id",
            session_id,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            kind,
            "--logical-name",
            logical_name,
            "--path",
            path,
        )
    )
    return data(
        run_cli(
            repo,
            "verdict",
            "--session-id",
            session_id,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            verdict,
            "--findings-artifact-id",
            str(artifact["artifact_id"]),
            *(["--rationale", rationale] if rationale is not None else []),
        )
    )


def test_init_status_and_doctor(tmp_path: Path) -> None:
    init_repo(tmp_path)
    assert (tmp_path / ".striatum" / "state.sqlite3").exists()
    assert ".striatum/" in (tmp_path / ".gitignore").read_text(encoding="utf-8")
    status = data(run_cli(tmp_path, "status"))
    assert status["runs"] == []
    doctor = data(run_cli(tmp_path, "doctor"))
    assert doctor["ok"] is True


def test_local_api_wraps_cli_semantics_without_printing_or_exiting(tmp_path: Path) -> None:
    initialized = api_data(invoke(["init"], repo=tmp_path))
    assert initialized["state_dir"] == str(tmp_path / ".striatum")
    status = api_data(invoke(["status"], repo=tmp_path))
    assert status["runs"] == []

    rejected = invoke(["claim-next", "--session-id", "missing"], repo=tmp_path)
    assert rejected["ok"] is False
    error = rejected["error"]
    assert isinstance(error, dict)
    assert error["code"] == 3


def test_local_mcp_wrapper_exposes_tools_and_delegates_to_api(tmp_path: Path) -> None:
    server = LocalRpcServer(repo=tmp_path)
    initialized = rpc_result(server, "initialize")
    assert initialized["serverInfo"] == {"name": "striatum-local", "version": "0.1.0"}

    tools = rpc_result(server, "tools/list")["tools"]
    assert isinstance(tools, list)
    assert any(tool["name"] == "status" for tool in tools)

    init_call = rpc_result(server, "tools/call", {"name": "init", "arguments": {}})
    init_result = init_call["structuredContent"]
    assert isinstance(init_result, dict)
    assert init_result["ok"] is True

    status_call = rpc_result(server, "tools/call", {"name": "status", "arguments": {}})
    status_result = status_call["structuredContent"]
    assert isinstance(status_result, dict)
    assert api_data(cast(JsonDict, status_result))["runs"] == []


def test_local_mcp_wrapper_supports_resources_and_raw_invoke(tmp_path: Path) -> None:
    server = LocalRpcServer(repo=tmp_path)
    init_repo(tmp_path)

    status_resource = rpc_result(server, "resources/read", {"uri": "striatum://status"})
    contents = status_resource["contents"]
    assert isinstance(contents, list)
    first = contents[0]
    assert isinstance(first, dict)
    resource_payload = json.loads(str(first["text"]))
    assert api_data(cast(JsonDict, resource_payload))["runs"] == []

    doctor = rpc_result(server, "striatum/invoke", {"args": ["doctor"]})
    assert api_data(doctor)["ok"] is True


def _frame(body: str) -> bytes:
    """Encode ``body`` as a single Content-Length-framed message."""
    encoded = body.encode("utf-8")
    return f"Content-Length: {len(encoded)}\r\n\r\n".encode("ascii") + encoded


def _split_framed_messages(payload: bytes) -> list[str]:
    """Decode one or more Content-Length-framed messages from ``payload``."""
    bodies: list[str] = []
    cursor = 0
    while cursor < len(payload):
        # Find the end of the header block.
        sep = payload.find(b"\r\n\r\n", cursor)
        assert sep != -1, f"missing CRLF CRLF in framed output at offset {cursor}: {payload!r}"
        header_block = payload[cursor:sep].decode("ascii")
        length: int | None = None
        for header in header_block.split("\r\n"):
            name, _, value = header.partition(":")
            if name.strip().lower() == "content-length":
                length = int(value.strip())
                break
        assert length is not None, f"missing Content-Length in {header_block!r}"
        body_start = sep + 4
        body_end = body_start + length
        bodies.append(payload[body_start:body_end].decode("utf-8"))
        cursor = body_end
    return bodies


def test_mcp_handles_content_length_framing(tmp_path: Path) -> None:
    request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize"})
    stdin = io.BytesIO(_frame(request))
    stdout = io.BytesIO()

    serve_stdio(repo=tmp_path, stdin=stdin, stdout=stdout)

    raw = stdout.getvalue()
    assert raw.startswith(b"Content-Length:"), f"expected framed output, got {raw!r}"
    bodies = _split_framed_messages(raw)
    assert len(bodies) == 1
    response = json.loads(bodies[0])
    assert response["id"] == 1
    assert response["jsonrpc"] == "2.0"
    assert response["result"]["serverInfo"] == {"name": "striatum-local", "version": "0.1.0"}


def test_mcp_handles_line_delimited_legacy(tmp_path: Path) -> None:
    request = json.dumps({"jsonrpc": "2.0", "id": 7, "method": "initialize"})
    stdin = io.BytesIO((request + "\n").encode("utf-8"))
    stdout = io.BytesIO()

    serve_stdio(repo=tmp_path, stdin=stdin, stdout=stdout)

    raw = stdout.getvalue()
    assert b"Content-Length" not in raw
    assert raw.endswith(b"\n")
    response = json.loads(raw.decode("utf-8").rstrip("\n"))
    assert response["id"] == 7
    assert response["result"]["serverInfo"]["name"] == "striatum-local"


def test_mcp_handles_two_framed_requests_in_sequence(tmp_path: Path) -> None:
    first = json.dumps({"jsonrpc": "2.0", "id": "a", "method": "initialize"})
    second = json.dumps({"jsonrpc": "2.0", "id": "b", "method": "tools/list"})
    stdin = io.BytesIO(_frame(first) + _frame(second))
    stdout = io.BytesIO()

    serve_stdio(repo=tmp_path, stdin=stdin, stdout=stdout)

    bodies = _split_framed_messages(stdout.getvalue())
    assert len(bodies) == 2
    parsed = [json.loads(body) for body in bodies]
    assert [item["id"] for item in parsed] == ["a", "b"]
    assert "serverInfo" in parsed[0]["result"]
    tools = parsed[1]["result"]["tools"]
    assert isinstance(tools, list)
    assert any(tool["name"] == "status" for tool in tools)


def test_mcp_handles_framed_body_with_embedded_newlines(tmp_path: Path) -> None:
    init_repo(tmp_path)
    # A multi-line request body is the whole point of Content-Length framing.
    payload: dict[str, Any] = {
        "jsonrpc": "2.0",
        "id": 11,
        "method": "tools/call",
        "params": {"name": "status", "arguments": {}},
    }
    request = json.dumps(payload, indent=2)
    assert "\n" in request

    stdin = io.BytesIO(_frame(request))
    stdout = io.BytesIO()

    serve_stdio(repo=tmp_path, stdin=stdin, stdout=stdout)

    bodies = _split_framed_messages(stdout.getvalue())
    assert len(bodies) == 1
    response = json.loads(bodies[0])
    assert response["id"] == 11
    structured = response["result"]["structuredContent"]
    assert structured["ok"] is True
    assert structured["data"]["runs"] == []


def test_workflow_validate_accepts_json_and_rejects_yaml(tmp_path: Path) -> None:
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(WORKFLOW)))
    assert valid["workflow_id"] == "rfc-ledger-cleanup"
    yaml_path = tmp_path / "workflow.yaml"
    yaml_path.write_text("schema_version: striatum.workflow.v1\n", encoding="utf-8")
    rejected = run_cli(tmp_path, "workflow", "validate", str(yaml_path), check=False)
    assert rejected["returncode"] == 8


def test_workflow_plan_explains_claim_order_and_review_gates(tmp_path: Path) -> None:
    plan = data(run_cli(tmp_path, "workflow", "plan", str(WORKFLOW)))
    assert plan["workflow_id"] == "rfc-ledger-cleanup"
    assert plan["summary"] == {"jobs": 6, "edges": 6, "cycles": 1, "claim_steps": 5}

    claim_order = plan["claim_order"]
    assert isinstance(claim_order, list)
    assert [step["step"] for step in claim_order] == [1, 2, 3, 4, 5]
    assert [job["job_id"] for job in claim_order[0]["claimable"]] == ["draft"]
    assert [job["job_id"] for job in claim_order[1]["claimable"]] == ["review_codex", "review_gemini"]
    assert [job["job_id"] for job in claim_order[4]["claimable"]] == ["final_review"]

    review_gates = {gate["review_job_id"]: gate for gate in plan["review_gates"]}
    assert review_gates["review_codex"]["downstream_jobs"] == ["findings_ledger"]
    assert review_gates["review_codex"]["accepting_verdicts"] == ["accept", "accept_with_findings"]
    assert review_gates["review_codex"]["needs_revision"] == {"action": "no_declared_route"}
    assert review_gates["final_review"]["needs_revision"] == {
        "action": "cycle",
        "to": "synthesis",
        "max_iterations": 1,
    }

    graph_edges = plan["graph"]["edges"]
    assert {
        "from": "review_codex",
        "to": "findings_ledger",
        "gate": {"on": "completed", "requires_verdict": ["accept", "accept_with_findings"]},
    } in graph_edges


def test_workflow_graph_exports_mermaid_and_json(tmp_path: Path) -> None:
    mermaid = run_cli_text(tmp_path, "workflow", "graph", str(WORKFLOW))
    assert mermaid.startswith("flowchart TD\n")
    assert '["draft<br/>draft author/codex"]' in mermaid
    assert 'subgraph pg0["parallel: reviews"]' in mermaid
    assert "-->|accepted review|" in mermaid
    assert "-.->|needs_revision max 1|" in mermaid

    graph = data(run_cli(tmp_path, "workflow", "graph", str(WORKFLOW), "--format", "json"))
    assert graph["workflow_id"] == "rfc-ledger-cleanup"
    graph_data = graph["graph"]
    assert isinstance(graph_data, dict)
    assert len(graph_data["nodes"]) == 6
    assert len(graph_data["edges"]) == 6
    assert graph_data["cycles"] == [
        {"from": "final_review", "to": "synthesis", "on_verdict": "needs_revision", "max_iterations": 1}
    ]
    review_node = next(node for node in graph_data["nodes"] if node["job_id"] == "review_codex")
    assert review_node["parallel_group"] == "reviews"


def test_docs_review_flow_fixture_validates_and_exports_graph(tmp_path: Path) -> None:
    valid = data(run_cli(tmp_path, "workflow", "validate", str(DOCS_REVIEW_WORKFLOW)))
    assert valid["workflow_id"] == "docs-review-flow"
    graph = data(run_cli(tmp_path, "workflow", "graph", str(DOCS_REVIEW_WORKFLOW), "--format", "json"))
    graph_data = graph["graph"]
    assert isinstance(graph_data, dict)
    assert [node["job_id"] for node in graph_data["nodes"]] == ["draft_docs", "review_docs", "apply_docs"]
    assert graph_data["cycles"] == []


def test_branch_confirmation_blocks_claims(tmp_path: Path) -> None:
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    session_id = register(tmp_path, run_id, "author", "codex")
    blocked = run_cli(tmp_path, "claim-next", "--session-id", session_id, check=False)
    assert blocked["returncode"] == 7
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    packet = claim(tmp_path, session_id)
    job = packet["job"]
    assert isinstance(job, dict)
    assert job["workflow_job_id"] == "draft"
    author = job["author"]
    assert isinstance(author, dict)
    assert author["line"] == "author: author-codex-gpt-5.5-001"
    assert "draft" not in str(author["line"])
    expected_artifacts = packet["expected_artifacts"]
    assert isinstance(expected_artifacts, list)
    first_artifact = expected_artifacts[0]
    assert isinstance(first_artifact, dict)
    assert first_artifact["author_line"] == author["line"]


def test_register_session_rejects_unknown_role_or_lane(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    bad_role = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "ghost",
        "--lane",
        "codex",
        check=False,
    )
    assert bad_role["returncode"] == 4
    bad_lane = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "author",
        "--lane",
        "ghost",
        check=False,
    )
    assert bad_lane["returncode"] == 4


def test_complete_requires_ack(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, _, lease_id = packet_ids(packet)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md")
    # Publishing is allowed for claimed work, but completion still requires ack.
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    rejected = run_cli(
        tmp_path,
        "complete",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4


def test_artifact_completion_and_verdict_flow(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    review_packet = claim(tmp_path, reviewer)
    job_id, message_id, lease_id = packet_ids(review_packet)
    run_cli(tmp_path, "ack", "--session-id", reviewer, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md")
    artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
        )
    )
    verdict = data(
        run_cli(
            tmp_path,
            "verdict",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            "accept",
            "--findings-artifact-id",
            str(artifact["artifact_id"]),
        )
    )
    assert verdict["status"] == "completed"
    why = data(run_cli(tmp_path, "why", job_id))
    events = why["events"]
    assert isinstance(events, list)
    assert any(event["event_type"] == "verdict.recorded" for event in events if isinstance(event, dict))


def test_release_requeues_fresh_review_for_new_session_only(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    review_packet = claim(tmp_path, reviewer)
    _, message_id, lease_id = packet_ids(review_packet)
    run_cli(
        tmp_path,
        "release",
        "--session-id",
        reviewer,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
        "--reason",
        "freshness test",
        "--requeue",
    )
    no_work = data(run_cli(tmp_path, "claim-next", "--session-id", reviewer))
    assert no_work["status"] == "no_work"
    replacement = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, replacement)
    job = packet["job"]
    assert isinstance(job, dict)
    assert job["workflow_job_id"] == "review_codex"


def test_publish_artifact_rejects_out_of_scope_paths(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "outside.md")
    rejected = run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "outside.md",
        check=False,
    )
    assert rejected["returncode"] == 6


def test_publish_artifact_validates_optional_markdown_author_line(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    path = "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md"

    write_artifact(
        tmp_path,
        path,
        text="# Draft\n\nAuthor: author-codex-gpt-5.5-001\n",
    )
    uppercase = run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        path,
        check=False,
    )
    assert uppercase["returncode"] == 6
    assert artifact_count(tmp_path, job_id) == 0

    write_artifact(
        tmp_path,
        path,
        text="---\nauthor: reviewer-codex-gpt-5.5-001\n---\n\n# Draft\n",
    )
    wrong_author = run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        path,
        check=False,
    )
    assert wrong_author["returncode"] == 6
    assert artifact_count(tmp_path, job_id) == 0

    write_artifact(
        tmp_path,
        path,
        text="# Draft\n\nStatus: draft\nDate: 2026-05-07\nauthor: author-codex-gpt-5.5-001\n",
    )
    published = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            "handoff",
            "--logical-name",
            "draft",
            "--path",
            path,
        )
    )
    assert published["status"] == "published"
    assert artifact_count(tmp_path, job_id) == 1


def test_decision_record_writes_run_level_decision_artifact_without_lease(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    recorded = data(
        run_cli(
            tmp_path,
            "decision",
            "record",
            "--run-id",
            run_id,
            "--path",
            "docs/decisions/owner-choice.md",
            "--decision-id",
            "owner-choice-001",
            "--outcome",
            "accepted_with_follow_up",
            "--title",
            "Keep decisions as durable artifacts",
            "--rationale",
            "Owner selected a local-first durable record.",
            "--follow-up",
            "Review fuller decision schemas later.",
        )
    )
    assert recorded["status"] == "recorded"
    assert recorded["outcome"] == "accepted_with_follow_up"

    text = (tmp_path / "docs" / "decisions" / "owner-choice.md").read_text(encoding="utf-8")
    assert "schema_version: striatum.decision.v1" in text
    assert 'decision_id: "owner-choice-001"' in text
    assert "artifact_kind: decision" in text
    assert "outcome: accepted_with_follow_up" in text
    assert "follow_up_required: true" in text

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        artifact = conn.execute(
            """
            SELECT artifact_kind, job_id, session_id, logical_name, repo_path
            FROM artifacts
            WHERE artifact_id = ?
            """,
            (recorded["artifact_id"],),
        ).fetchone()
        assert artifact == (
            "decision",
            None,
            None,
            "owner-choice-001",
            "docs/decisions/owner-choice.md",
        )
        event = conn.execute(
            """
            SELECT event_type, artifact_id, payload_json
            FROM events
            WHERE artifact_id = ?
            """,
            (recorded["artifact_id"],),
        ).fetchone()
        assert event is not None
    finally:
        conn.close()
    assert event[0] == "decision.recorded"
    assert event[1] == recorded["artifact_id"]
    assert json.loads(event[2])["outcome"] == "accepted_with_follow_up"


def test_events_are_append_only(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        event_id = conn.execute("SELECT event_id FROM events WHERE run_id = ? LIMIT 1", (run_id,)).fetchone()[0]
        try:
            conn.execute("UPDATE events SET event_type = 'tampered' WHERE event_id = ?", (event_id,))
        except sqlite3.DatabaseError as exc:
            assert "append-only" in str(exc)
        else:
            raise AssertionError("events update unexpectedly succeeded")
    finally:
        conn.close()


def test_verdict_reject_fails_run_and_does_not_enqueue_downstream(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="reject",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    assert verdict["status"] == "failed"
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "failed"
    assert status["jobs"]["failed"] == 1
    ledger = register(tmp_path, run_id, "ledger", "codex")
    no_work = data(run_cli(tmp_path, "claim-next", "--session-id", ledger))
    assert no_work["status"] == "no_work"


def test_accepting_review_verdict_unblocks_downstream(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    codex = register(tmp_path, run_id, "reviewer", "codex")
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        codex,
        claim(tmp_path, codex),
        verdict="accept_with_findings",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    ledger = register(tmp_path, run_id, "ledger", "codex")
    assert data(run_cli(tmp_path, "claim-next", "--session-id", ledger))["status"] == "no_work"
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )
    packet = claim(tmp_path, ledger)
    assert packet["job"]["workflow_job_id"] == "findings_ledger"


def test_verdict_needs_revision_uses_declared_cycle(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    codex = register(tmp_path, run_id, "reviewer", "codex")
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        codex,
        claim(tmp_path, codex),
        verdict="accept",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )
    ledger = register(tmp_path, run_id, "ledger", "codex")
    complete_claimed_job(
        tmp_path,
        ledger,
        claim(tmp_path, ledger),
        logical_name="ledger",
        kind="findings_ledger",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_FINDINGS_LEDGER.md",
    )
    synth = register(tmp_path, run_id, "synthesizer", "claude")
    complete_claimed_job(
        tmp_path,
        synth,
        claim(tmp_path, synth),
        logical_name="synthesis",
        kind="synthesis",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_SYNTHESIS.md",
    )
    final = register(tmp_path, run_id, "reviewer", "claude")
    verdict = verdict_claimed_review(
        tmp_path,
        final,
        claim(tmp_path, final),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/final/RFC_LEDGER_FINAL_REVIEW.md",
    )
    assert verdict["status"] == "revision_requested"
    next_synth = register(tmp_path, run_id, "synthesizer", "claude")
    packet = claim(tmp_path, next_synth)
    assert packet["job"]["workflow_job_id"] == "synthesis"
    assert packet["job"]["attempt"] == 2
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "running"


def test_verdict_needs_revision_without_cycle_waits_human(tmp_path: Path) -> None:
    workflow = example_workflow()
    workflow["cycles"] = []
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    assert verdict["status"] == "waiting_human"
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        blocker = conn.execute("SELECT state FROM blockers WHERE run_id = ?", (run_id,)).fetchone()
        assert blocker[0] == "open"
    finally:
        conn.close()
    ledger = register(tmp_path, run_id, "ledger", "codex")
    assert data(run_cli(tmp_path, "claim-next", "--session-id", ledger))["status"] == "no_work"


def test_edges_materialize_dependencies_without_needs(tmp_path: Path) -> None:
    workflow = example_workflow()
    for job in workflow["jobs"]:
        job.pop("needs", None)
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    author = register(tmp_path, run_id, "author", "codex")
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    assert claim(tmp_path, author)["job"]["workflow_job_id"] == "draft"
    assert data(run_cli(tmp_path, "claim-next", "--session-id", reviewer))["status"] == "no_work"


def test_workflow_rejects_needs_edges_mismatch(tmp_path: Path) -> None:
    init_repo(tmp_path)
    workflow = example_workflow()
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    mismatched = deepcopy(workflow)
    mismatched["jobs"][1]["needs"] = []
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, mismatched)), check=False)
    assert rejected["returncode"] == 8


def test_complete_requires_expected_artifact_path_and_kind(tmp_path: Path) -> None:
    bad_repo = tmp_path / "bad"
    bad_repo.mkdir()
    run_id = prepare_started_run(bad_repo)
    author = register(bad_repo, run_id, "author", "codex")
    packet = claim(bad_repo, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(bad_repo, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(bad_repo, "docs/reviews/rfc-ledger/WRONG.md")
    run_cli(
        bad_repo,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "docs/reviews/rfc-ledger/WRONG.md",
    )
    rejected = run_cli(
        bad_repo,
        "complete",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4

    good_repo = tmp_path / "good"
    good_repo.mkdir()
    run_id = prepare_started_run(good_repo)
    author = register(good_repo, run_id, "author", "codex")
    complete_claimed_job(
        good_repo,
        author,
        claim(good_repo, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )


def test_verdict_requires_expected_artifact_path_and_kind(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    job_id, message_id, lease_id = packet_ids(packet)
    run_why = data(run_cli(tmp_path, "why", run_id))
    assert run_why["target_type"] == "run"
    session_why = data(run_cli(tmp_path, "why", reviewer))
    assert session_why["target_type"] == "session"
    message_why = data(run_cli(tmp_path, "why", message_id))
    assert message_why["target_type"] == "message"
    run_cli(tmp_path, "ack", "--session-id", reviewer, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/WRONG.md")
    artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/reviews/rfc-ledger/codex/WRONG.md",
        )
    )
    rejected = run_cli(
        tmp_path,
        "verdict",
        "--session-id",
        reviewer,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--verdict",
        "accept",
        "--findings-artifact-id",
        str(artifact["artifact_id"]),
        check=False,
    )
    assert rejected["returncode"] == 4


def test_doctor_reports_bad_review_gate_state(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        review = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? AND workflow_job_id = 'review_codex'",
            (run_id,),
        ).fetchone()
        conn.execute("UPDATE jobs SET state = 'completed' WHERE job_id = ?", (review[0],))
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any("lacks accepting verdict" in problem for problem in doctor["problems"])


def test_blocked_review_verdict_appears_in_status(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert verdict["blocker_id"] == status["human_checkpoints"][0]["blocker_id"]
    checkpoint = status["human_checkpoints"][0]["human_checkpoint"]
    assert checkpoint["decision_required"].startswith("Human decision required")
    assert any(job["workflow_job_id"] == "findings_ledger" for job in checkpoint["affected_jobs"])
    assert "resume_or_requeue_affected_work" in checkpoint["unblock_path"]
    assert status["latest_non_accepting_review_verdicts"][0]["verdict"] == "needs_revision"
    assert any(job["workflow_job_id"] == "findings_ledger" for job in status["blocked_downstream_jobs"])
    assert "resolve_human_checkpoint" in status["next_actions"]


def test_why_resolves_blocker_artifact_and_verdict(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(tmp_path, "ack", "--session-id", reviewer, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md")
    artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
        )
    )
    verdict = data(
        run_cli(
            tmp_path,
            "verdict",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            "needs_revision",
            "--findings-artifact-id",
            str(artifact["artifact_id"]),
        )
    )
    blocker = data(run_cli(tmp_path, "why", str(verdict["blocker_id"])))
    assert blocker["target_type"] == "blocker"
    assert blocker["related_verdict"]["verdict"] == "needs_revision"
    assert blocker["human_checkpoint"]["decision_required"].startswith("Human decision required")
    assert "review_related_verdict_and_artifact" in blocker["human_checkpoint"]["unblock_path"]
    assert any(job["workflow_job_id"] == "findings_ledger" for job in blocker["blocked_downstream_jobs"])
    artifact_why = data(run_cli(tmp_path, "why", str(artifact["artifact_id"])))
    assert artifact_why["target_type"] == "artifact"
    assert artifact_why["verdicts"][0]["verdict_id"] == verdict["verdict_id"]
    verdict_why = data(run_cli(tmp_path, "why", str(verdict["verdict_id"])))
    assert verdict_why["target_type"] == "verdict"
    assert verdict_why["artifact"]["artifact_id"] == artifact["artifact_id"]


def test_evidence_export_writes_redacted_markdown_and_rejects_bad_paths(tmp_path: Path) -> None:
    private_job_title = "PRIVATE_JOB_TITLE_corpus_project_alpha"
    workflow = example_workflow()
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    first_job = jobs[0]
    assert isinstance(first_job, dict)
    first_job["title"] = private_job_title
    workflow_path = temporary_workflow(tmp_path, workflow)
    run_id = prepare_started_run(tmp_path, workflow_path=workflow_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
        rationale="private corpus excerpt from /tmp/private-notes",
    )
    exported = data(
        run_cli(
            tmp_path,
            "evidence",
            "export",
            "--run-id",
            run_id,
            "--path",
            "docs/reviews/rfc-ledger/RUN_EVIDENCE.md",
        )
    )
    assert exported["status"] == "exported"
    evidence = (tmp_path / "docs/reviews/rfc-ledger/RUN_EVIDENCE.md").read_text(encoding="utf-8")
    assert "Striatum Evidence Export" in evidence
    assert "needs_revision" in evidence
    assert "private corpus excerpt" not in evidence
    assert "/tmp/private-notes" not in evidence
    assert private_job_title not in evidence
    assert '"title"' not in evidence
    assert "author: reviewer-codex-gpt-5.5-001" in evidence
    assert "Author:" not in evidence
    assert "<redacted-free-text>" in evidence
    assert "state.sqlite3" not in evidence
    assert "transcript" not in evidence.lower()
    bad_state = run_cli(
        tmp_path,
        "evidence",
        "export",
        "--run-id",
        run_id,
        "--path",
        ".striatum/RUN_EVIDENCE.md",
        check=False,
    )
    assert bad_state["returncode"] == 6
    bad_escape = run_cli(
        tmp_path,
        "evidence",
        "export",
        "--run-id",
        run_id,
        "--path",
        "../RUN_EVIDENCE.md",
        check=False,
    )
    assert bad_escape["returncode"] == 6


def test_run_summary_export_writes_compact_note(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    exported = data(
        run_cli(
            tmp_path,
            "run",
            "summary",
            "--run-id",
            run_id,
            "--path",
            "docs/reviews/rfc-ledger/RUN_SUMMARY.md",
        )
    )
    assert exported["status"] == "exported"
    summary = (tmp_path / "docs/reviews/rfc-ledger/RUN_SUMMARY.md").read_text(encoding="utf-8")
    assert "Striatum Run Summary" in summary
    assert f"Run ID: `{run_id}`" in summary
    assert "Verification: `doctor ok=true`" in summary
    assert "`needs_revision` on `review_codex`" in summary
    assert "`finding` `review`: `docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md`" in summary
    assert "`human_checkpoint`" in summary


def test_recovery_stale_leases_reports_repo_write_policy(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = data(run_cli(tmp_path, "claim-next", "--session-id", author, "--lease-seconds", "-1"))["packet"]
    assert isinstance(packet, dict)
    job_id, _message_id, lease_id = packet_ids(packet)
    recovery = data(run_cli(tmp_path, "recovery", "stale-leases", "--run-id", run_id))
    assert recovery["stale_count"] == 1
    stale = recovery["stale_leases"][0]
    assert stale["job_id"] == job_id
    assert stale["lease_id"] == lease_id
    assert stale["workflow_job_id"] == "draft"
    assert stale["repo_write"] is True
    assert stale["recovery_policy"] == "manual_inspection_required"
    assert "inspect_worktree_and_artifacts" in stale["next_actions"]


def test_recovery_requeue_stale_rejects_repo_write_jobs(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = data(run_cli(tmp_path, "claim-next", "--session-id", author, "--lease-seconds", "-1"))["packet"]
    assert isinstance(packet, dict)
    job_id, _message_id, _lease_id = packet_ids(packet)

    rejected = run_cli(
        tmp_path,
        "recovery",
        "requeue-stale",
        "--run-id",
        run_id,
        "--job-id",
        job_id,
        check=False,
    )
    assert rejected["returncode"] == 4
    assert rejected["error"]["message"] == "repo-write stale jobs require manual inspection"


def test_recovery_requeue_stale_allows_review_only_jobs(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = data(run_cli(tmp_path, "claim-next", "--session-id", reviewer, "--lease-seconds", "-1"))["packet"]
    assert isinstance(packet, dict)
    job_id, _message_id, lease_id = packet_ids(packet)

    recovery = data(
        run_cli(
            tmp_path,
            "recovery",
            "requeue-stale",
            "--run-id",
            run_id,
            "--job-id",
            job_id,
        )
    )
    assert recovery["status"] == "already_reclaimable"
    assert recovery["job_id"] == job_id
    assert recovery["lease_id"] == lease_id
    assert recovery["repo_write"] is False

    next_reviewer = register(tmp_path, run_id, "reviewer", "codex")
    reclaimed = data(run_cli(tmp_path, "claim-next", "--session-id", next_reviewer))["packet"]
    assert isinstance(reclaimed, dict)
    assert reclaimed["job"]["job_id"] == job_id


def test_submit_review_publishes_artifact_and_applies_gate(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    job_id, _message_id, lease_id = packet_ids(packet)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md", text="needs revision\n")
    submitted = data(
        run_cli(
            tmp_path,
            "submit-review",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--path",
            "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
            "--verdict",
            "needs_revision",
            "--rationale",
            "root review blocks",
        )
    )
    assert submitted["artifact"]["status"] == "published"
    assert submitted["verdict"]["verdict"] == "needs_revision"
    assert submitted["blocker_id"] == submitted["verdict"]["blocker_id"]
    assert submitted["job_state"] == "waiting_human"
    assert any(job["workflow_job_id"] == "findings_ledger" for job in submitted["downstream_jobs"])


def test_submit_review_prevalidates_before_publishing_artifact(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    job_id, _message_id, lease_id = packet_ids(packet)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/WRONG_REVIEW.md", text="wrong\n")
    rejected = run_cli(
        tmp_path,
        "submit-review",
        "--session-id",
        reviewer,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--path",
        "docs/reviews/rfc-ledger/codex/WRONG_REVIEW.md",
        "--logical-name",
        "wrong",
        "--kind",
        "other",
        "--verdict",
        "needs_revision",
        check=False,
    )
    assert rejected["returncode"] == 4
    assert artifact_count(tmp_path, job_id) == 0

    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md", text="correct\n")
    submitted = data(
        run_cli(
            tmp_path,
            "submit-review",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--path",
            "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
            "--verdict",
            "needs_revision",
        )
    )
    assert submitted["artifact"]["status"] == "published"
    assert artifact_count(tmp_path, job_id) == 1


def test_submit_review_rejects_non_review_before_publishing_artifact(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, _message_id, lease_id = packet_ids(packet)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md", text="draft\n")
    rejected = run_cli(
        tmp_path,
        "submit-review",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--path",
        "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
        "--logical-name",
        "draft",
        "--kind",
        "handoff",
        "--verdict",
        "accept",
        check=False,
    )
    assert rejected["returncode"] == 4
    assert artifact_count(tmp_path, job_id) == 0


def test_workflow_lane_constraints_validate_and_appear_in_packets(tmp_path: Path) -> None:
    workflow = example_workflow()
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    codex = lanes["codex"]
    assert isinstance(codex, dict)
    codex["constraints"] = {
        "network": "forbidden",
        "transcripts": "off",
        "repo_scope": "local_only",
    }
    codex["required_enforcement"] = {
        "network": "advisory",
        "transcripts": "enforced",
    }
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    run_cli(tmp_path, "workflow", "validate", str(workflow_path))
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    constraints = packet["adapter_constraints"]
    assert isinstance(constraints, dict)
    assert constraints["requested"]["network"] == "forbidden"
    assert constraints["required_enforcement"]["transcripts"] == "enforced"
    assert constraints["satisfied"] is True
    assert {
        "constraint": "network",
        "requested": "forbidden",
        "required_enforcement": "advisory",
        "enforcement": "advisory",
        "satisfied": True,
    } in constraints["enforcement"]
    assert {
        "constraint": "transcripts",
        "requested": "off",
        "required_enforcement": "enforced",
        "enforcement": "enforced",
        "satisfied": True,
    } in constraints["enforcement"]
    invalid = example_workflow()
    invalid_lanes = invalid["lanes"]
    assert isinstance(invalid_lanes, dict)
    invalid_codex = invalid_lanes["codex"]
    assert isinstance(invalid_codex, dict)
    invalid_codex["constraints"] = {"network": "maybe"}
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, invalid)), check=False)
    assert rejected["returncode"] == 8

    unmet = example_workflow()
    unmet_lanes = unmet["lanes"]
    assert isinstance(unmet_lanes, dict)
    unmet_codex = unmet_lanes["codex"]
    assert isinstance(unmet_codex, dict)
    unmet_codex["constraints"] = {"network": "forbidden"}
    unmet_codex["required_enforcement"] = {"network": "enforced"}
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, unmet)), check=False)
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "requires 'enforced' enforcement" in str(error["message"])

    undeclared = example_workflow()
    undeclared_lanes = undeclared["lanes"]
    assert isinstance(undeclared_lanes, dict)
    undeclared_codex = undeclared_lanes["codex"]
    assert isinstance(undeclared_codex, dict)
    undeclared_codex["required_enforcement"] = {"network": "advisory"}
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, undeclared)), check=False)
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "requires enforcement for undeclared constraint" in str(error["message"])


def test_process_adapter_runs_configured_command_and_records_metadata(tmp_path: Path) -> None:
    workflow = example_workflow()
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    codex = lanes["codex"]
    assert isinstance(codex, dict)
    codex["command"] = [
        sys.executable,
        "-c",
        (
            "import os, pathlib, sys; "
            "packet = sys.stdin.read(); "
            "scratch = pathlib.Path(os.environ['STRIATUM_SCRATCH_DIR']); "
            "scratch.mkdir(parents=True, exist_ok=True); "
            "(scratch / 'packet.json').write_text(packet, encoding='utf-8'); "
            "pathlib.Path('adapter-ran.txt').write_text("
            "os.environ['STRIATUM_PROCESS_ID'], encoding='utf-8'"
            ")"
        ),
    ]
    workflow_path = temporary_workflow(tmp_path, workflow)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    _, _, lease_id = packet_ids(packet)

    result = data(
        run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            author,
            "--lease-id",
            lease_id,
        )
    )

    assert result["state"] == "exited"
    assert result["exit_code"] == 0
    process_id = str(result["process_id"])
    assert (tmp_path / "adapter-ran.txt").read_text(encoding="utf-8") == process_id
    scratch_packet = Path(str(result["scratch_path"])) / "packet.json"
    assert json.loads(scratch_packet.read_text(encoding="utf-8"))["packet_id"] == packet["packet_id"]

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            """
            SELECT process_id, state, exit_code, command_json
            FROM process_executions WHERE process_id = ?
            """,
            (process_id,),
        ).fetchone()
        events = conn.execute(
            """
            SELECT event_type, payload_json FROM events
            WHERE event_type LIKE 'process.%'
            ORDER BY event_id
            """
        ).fetchall()
    finally:
        conn.close()
    assert row == (process_id, "exited", 0, json.dumps(codex["command"], sort_keys=True, separators=(",", ":")))
    process_events = [event for event in events if json.loads(event[1]).get("process_id") == process_id]
    assert [event[0] for event in process_events] == [
        "process.starting",
        "process.started",
        "process.exited",
    ]

    explained = data(run_cli(tmp_path, "why", process_id))
    assert explained["target_type"] == "process"
    assert len(explained["events"]) == 3


def test_branch_confirm_reports_records_only_and_mismatch(tmp_path: Path) -> None:
    init_repo(tmp_path)
    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "actual"], cwd=tmp_path, check=True, capture_output=True)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    confirmed = data(
        run_cli(
            tmp_path,
            "branch",
            "confirm",
            "--run-id",
            str(prepared["run_id"]),
            "--branch",
            "expected",
        )
    )
    assert confirmed["records_only"] is True
    assert confirmed["requested_branch"] == "expected"
    assert confirmed["current_git_branch"] == "actual"
    assert confirmed["warning"] is not None


def test_rfc_0014_fixture_declares_root_review_revision_policy(tmp_path: Path) -> None:
    fixture = ROOT / "examples" / "rfc-0014-operational-artifact-home" / "workflow.json"
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(fixture)))
    assert valid["workflow_id"] == "rfc-0014-operational-artifact-home"
    workflow = json.loads(fixture.read_text(encoding="utf-8"))
    policy = workflow["review_revision_policy"]
    assert policy["root_review_needs_revision"] == "human_checkpoint"


def test_declared_cycle_policy_requires_root_review_cycles(tmp_path: Path) -> None:
    fixture = ROOT / "examples" / "rfc-0014-operational-artifact-home" / "workflow.json"
    workflow = json.loads(fixture.read_text(encoding="utf-8"))
    workflow["review_revision_policy"] = {"root_review_needs_revision": "declared_cycle"}
    init_repo(tmp_path)
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, workflow)), check=False)
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "declared_cycle" in str(error["message"])

    workflow["cycles"].extend(
        [
            {"from": "review_claude", "to": "findings_ledger", "on_verdict": "needs_revision", "max_iterations": 1},
            {"from": "review_codex", "to": "findings_ledger", "on_verdict": "needs_revision", "max_iterations": 1},
            {"from": "review_gemini", "to": "findings_ledger", "on_verdict": "needs_revision", "max_iterations": 1},
        ]
    )
    valid = data(run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, workflow))))
    assert valid["workflow_id"] == "rfc-0014-operational-artifact-home"


def test_doctor_flags_orphaned_message_and_lease_pointers(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        job_row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? AND workflow_job_id = 'draft'",
            (run_id,),
        ).fetchone()
        assert job_row is not None
        job_id = str(job_row[0])
        conn.execute(
            "UPDATE jobs SET current_message_id = 'msg-does-not-exist', current_lease_id = 'lease-does-not-exist' WHERE job_id = ?",
            (job_id,),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    problems = doctor["problems"]
    assert isinstance(problems, list)
    assert doctor["ok"] is False
    assert any(
        f"job current_message_id is inconsistent: {job_id}" == problem for problem in problems
    )
    assert any(
        f"job current_lease_id is inconsistent: {job_id}" == problem for problem in problems
    )


def test_doctor_flags_active_session_on_completed_run(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE runs SET state = 'completed', completed_at = ? WHERE run_id = ?",
            ("2026-05-07T00:00:00Z", run_id),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any(
        problem == f"active session on terminal run: {session_id}"
        for problem in doctor["problems"]
    )


def test_doctor_flags_unreaped_expired_leases(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    _, _, lease_id = packet_ids(packet)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET expires_at = ? WHERE lease_id = ?",
            ("2000-01-01T00:00:00Z", lease_id),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any(
        problem == f"active lease has expired without reap: {lease_id}"
        for problem in doctor["problems"]
    )


def test_doctor_flags_open_blocker_on_canceled_run(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        blocker = conn.execute(
            "SELECT blocker_id, state FROM blockers WHERE run_id = ?", (run_id,)
        ).fetchone()
        assert blocker is not None
        assert blocker[1] == "open"
        blocker_id = str(blocker[0])
        conn.execute(
            "UPDATE runs SET state = 'canceled', completed_at = ?, stop_reason = 'test' WHERE run_id = ?",
            ("2026-05-07T00:00:00Z", run_id),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any(
        problem == f"open blocker on terminal run: {blocker_id}"
        for problem in doctor["problems"]
    )


def test_doctor_flags_stale_queue_message_claim(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    _, message_id, lease_id = packet_ids(packet)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'test' WHERE lease_id = ?",
            ("2026-05-07T00:00:00Z", lease_id),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any(
        problem == f"queue message has stale claim: {message_id}"
        for problem in doctor["problems"]
    )


def test_events_for_process_does_not_leak_across_runs(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, _, lease_id = packet_ids(packet)
    session_id = author
    packet_id = str(packet["packet_id"])

    other_run_id = "run-other-doctor-test"
    other_job_id = "job-other-doctor-test"
    other_session_id = "session-other-doctor-test"
    other_lease_id = "lease-other-doctor-test"
    other_packet_id = "packet-other-doctor-test"
    other_workflow_snapshot_id = "snap-other-doctor-test"
    process_a = "process-a-doctor-test"
    process_b = "process-b-doctor-test"

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    conn.row_factory = sqlite3.Row
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        first_snapshot = conn.execute(
            "SELECT workflow_snapshot_id FROM runs WHERE run_id = ?", (run_id,)
        ).fetchone()
        assert first_snapshot is not None
        snapshot_row = conn.execute(
            "SELECT workflow_id, workflow_version, source_path, content_sha256, workflow_json, loaded_at FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
            (first_snapshot[0],),
        ).fetchone()
        assert snapshot_row is not None
        conn.execute(
            "INSERT INTO workflow_snapshots(workflow_snapshot_id, workflow_id, workflow_version, source_path, content_sha256, workflow_json, loaded_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
            (
                other_workflow_snapshot_id,
                snapshot_row["workflow_id"],
                snapshot_row["workflow_version"],
                snapshot_row["source_path"],
                snapshot_row["content_sha256"],
                snapshot_row["workflow_json"],
                snapshot_row["loaded_at"],
            ),
        )
        conn.execute(
            "INSERT INTO runs(run_id, workflow_snapshot_id, repo_root, state, branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at, started_at) VALUES (?, ?, ?, 'running', 'striatum/v1-test', NULL, ?, 'system', ?, ?)",
            (
                other_run_id,
                other_workflow_snapshot_id,
                str(tmp_path),
                "2026-05-07T00:00:00Z",
                "2026-05-07T00:00:00Z",
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO sessions(session_id, run_id, role_id, lane_id, slug, ordinal, capabilities_json, parent_session_id, first_class, fresh_context, state, registered_at) VALUES (?, ?, 'author', 'codex', 'author-codex-other', 1, '[]', NULL, 1, 0, 'active', ?)",
            (other_session_id, other_run_id, "2026-05-07T00:00:00Z"),
        )
        conn.execute(
            "INSERT INTO jobs(job_id, run_id, workflow_job_id, title, job_type, role_id, lane_selector_json, capability_requirements_json, state, attempt, max_attempts, fresh_session_required, write_scope_json, expected_artifacts_json, idempotency_key, created_at) VALUES (?, ?, 'draft-other', 'Draft other', 'draft', 'author', '{}', '[]', 'running', 1, 1, 0, '{}', '[]', 'idem-other', ?)",
            (other_job_id, other_run_id, "2026-05-07T00:00:00Z"),
        )
        conn.execute(
            "INSERT INTO leases(lease_id, run_id, resource_type, resource_id, owner_session_id, state, acquired_at, expires_at) VALUES (?, ?, 'job', ?, ?, 'active', ?, ?)",
            (
                other_lease_id,
                other_run_id,
                other_job_id,
                other_session_id,
                "2026-05-07T00:00:00Z",
                "2999-01-01T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO queue_messages(message_id, run_id, job_id, kind, state, priority, payload_json, created_at, updated_at) VALUES (?, ?, ?, 'work', 'claimed', 0, '{}', ?, ?)",
            (
                "msg-other-doctor-test",
                other_run_id,
                other_job_id,
                "2026-05-07T00:00:00Z",
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO work_packets(packet_id, run_id, job_id, message_id, lease_id, session_id, packet_json, packet_sha256, created_at) VALUES (?, ?, ?, 'msg-other-doctor-test', ?, ?, '{}', '0', ?)",
            (
                other_packet_id,
                other_run_id,
                other_job_id,
                other_lease_id,
                other_session_id,
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO process_executions(process_id, run_id, job_id, session_id, lease_id, packet_id, adapter, command_json, cwd, scratch_path, stdin_mode, stdio_mode, state, started_at) VALUES (?, ?, ?, ?, ?, ?, 'subprocess', '[]', ?, ?, 'packet', 'suppressed', 'running', ?)",
            (
                process_a,
                run_id,
                job_id,
                session_id,
                lease_id,
                packet_id,
                str(tmp_path),
                str(tmp_path),
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO process_executions(process_id, run_id, job_id, session_id, lease_id, packet_id, adapter, command_json, cwd, scratch_path, stdin_mode, stdio_mode, state, started_at) VALUES (?, ?, ?, ?, ?, ?, 'subprocess', '[]', ?, ?, 'packet', 'suppressed', 'running', ?)",
            (
                process_b,
                other_run_id,
                other_job_id,
                other_session_id,
                other_lease_id,
                other_packet_id,
                str(tmp_path),
                str(tmp_path),
                "2026-05-07T00:00:00Z",
            ),
        )
        # Both processes get a 'process.starting' event under the same event_type.
        conn.execute(
            "INSERT INTO events(run_id, event_type, actor_session_id, job_id, lease_id, payload_json, created_at) VALUES (?, 'process.starting', ?, ?, ?, ?, ?)",
            (
                run_id,
                session_id,
                job_id,
                lease_id,
                json.dumps({"process_id": process_a}),
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.execute(
            "INSERT INTO events(run_id, event_type, actor_session_id, job_id, lease_id, payload_json, created_at) VALUES (?, 'process.starting', ?, ?, ?, ?, ?)",
            (
                other_run_id,
                other_session_id,
                other_job_id,
                other_lease_id,
                json.dumps({"process_id": process_b}),
                "2026-05-07T00:00:00Z",
            ),
        )
        conn.commit()
    finally:
        conn.close()

    why_a = data(run_cli(tmp_path, "why", process_a))
    assert why_a["target_type"] == "process"
    events_a = why_a["events"]
    assert isinstance(events_a, list)
    assert len(events_a) == 1
    payload_a = json.loads(events_a[0]["payload_json"])
    assert payload_a["process_id"] == process_a

    why_b = data(run_cli(tmp_path, "why", process_b))
    assert why_b["target_type"] == "process"
    events_b = why_b["events"]
    assert isinstance(events_b, list)
    assert len(events_b) == 1
    payload_b = json.loads(events_b[0]["payload_json"])
    assert payload_b["process_id"] == process_b
