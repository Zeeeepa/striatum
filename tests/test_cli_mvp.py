from __future__ import annotations

import io
import json
from contextlib import redirect_stderr
import os
import shutil
import sqlite3
import subprocess
import sys
from copy import deepcopy
from pathlib import Path
from typing import Any, cast

import pytest

from striatum.api import invoke
from striatum.mcp import LocalRpcServer, serve_stdio

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"
DOCS_REVIEW_WORKFLOW = ROOT / "examples" / "docs-review-flow" / "workflow.json"
CODE_CHANGE_WORKFLOW = ROOT / "examples" / "code-change-flow" / "workflow.json"
FAILED_REVIEW_WORKFLOW = (
    ROOT / "examples" / "failed-review-revision-cycle" / "workflow.json"
)
HUMAN_CHECKPOINT_WORKFLOW = (
    ROOT / "examples" / "human-checkpoint-flow" / "workflow.json"
)
ADAPTER_UNAVAILABLE_WORKFLOW = (
    ROOT / "examples" / "adapter-unavailable-flow" / "workflow.json"
)


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
    args: list[str] = [
        "register-session",
        "--run-id",
        run_id,
        "--role",
        role,
        "--lane",
        lane,
        "--capability",
        "review",
    ]
    # HARNESS-003: when registering a reviewer in a workflow that
    # declares ``reviewer_context_policy: fresh`` and an active author
    # session already exists, the runner now refuses unless an explicit
    # override is provided. Tests that drive both lanes from the same
    # operator pass the override with a fixed test reason.
    if role == "reviewer":
        args += ["--force-non-fresh", "--reason", "test fixture"]
    payload = data(run_cli(repo, *args))
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


def _packet_default_artifact_body(packet: JsonDict, logical_name: str) -> str:
    """Build a default artifact body that carries the workflow-declared byline.

    Looks up ``expected_artifacts`` in the packet for the matching
    ``logical_name`` and prepends an ``author: ...`` title block when the
    packet declared one. Tests that want to exercise the missing-byline
    case skip this helper and write their own body.
    """
    expected = packet.get("expected_artifacts")
    if isinstance(expected, list):
        for item in expected:
            if not isinstance(item, dict):
                continue
            if item.get("logical_name") != logical_name:
                continue
            byline = item.get("author_line")
            if isinstance(byline, str) and byline.strip():
                return f"{byline.strip()}\n\nartifact\n"
            break
    return "artifact\n"


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
    # HARNESS-003: write the workflow-declared byline into the artifact
    # body so the new author_line column is populated and evidence
    # exports preserve model labels in the snapshot. Tests that
    # specifically exercise the missing-byline path can pass their own
    # body via the lower-level write_artifact + publish-artifact pair.
    write_artifact(repo, path, text=_packet_default_artifact_body(packet, logical_name))
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
    body = _packet_default_artifact_body(packet, logical_name) + f"{verdict}\n"
    write_artifact(repo, path, text=body)
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


def test_workflow_graph_exports_dot(tmp_path: Path) -> None:
    dot_text = run_cli_text(tmp_path, "workflow", "graph", str(WORKFLOW), "--format", "dot")
    assert dot_text.startswith("digraph striatum_workflow {\n")
    assert dot_text.rstrip().endswith("}")
    workflow_data = json.loads(WORKFLOW.read_text(encoding="utf-8"))
    for job in workflow_data["jobs"]:
        assert job["id"] in dot_text
    # Parallel group becomes a cluster_<group> subgraph with a label attribute.
    assert "subgraph cluster_reviews {" in dot_text
    assert 'label="parallel: reviews";' in dot_text
    # Dependency edges are solid arrows; review-acceptance edges carry the
    # "accepted review" label.
    assert "->" in dot_text
    assert '[label="completed"]' in dot_text
    assert '[label="accepted review"]' in dot_text
    # The needs_revision cycle becomes a dashed edge with max_iterations on
    # the label.
    assert "style=dashed" in dot_text
    assert "needs_revision max 1" in dot_text

    # JSON wrapper exposes the same DOT body under {"format":"dot","source":...}.
    wrapped = data(run_cli(tmp_path, "workflow", "graph", str(WORKFLOW), "--format", "dot", "--json"))
    assert wrapped["format"] == "dot"
    assert wrapped["source"].rstrip("\n") == dot_text.rstrip("\n")

    # If Graphviz is installed locally, the output must parse and render.
    if shutil.which("dot") is not None:
        proc = subprocess.run(
            ["dot", "-Tsvg", "-o", str(tmp_path / "workflow_graph.svg")],
            input=dot_text,
            capture_output=True,
            text=True,
        )
        assert proc.returncode == 0, proc.stderr


def test_docs_review_flow_fixture_validates_and_exports_graph(tmp_path: Path) -> None:
    valid = data(run_cli(tmp_path, "workflow", "validate", str(DOCS_REVIEW_WORKFLOW)))
    assert valid["workflow_id"] == "docs-review-flow"
    graph = data(run_cli(tmp_path, "workflow", "graph", str(DOCS_REVIEW_WORKFLOW), "--format", "json"))
    graph_data = graph["graph"]
    assert isinstance(graph_data, dict)
    assert [node["job_id"] for node in graph_data["nodes"]] == ["draft_docs", "review_docs", "apply_docs"]
    assert graph_data["cycles"] == []


def test_run_graph_highlights_job_states_in_mermaid(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path, DOCS_REVIEW_WORKFLOW)
    author = register(tmp_path, run_id, "author", "local")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/docs-review-flow/DOCS_DRAFT.md",
    )

    mermaid = run_cli_text(
        tmp_path,
        "run",
        "graph",
        "--run-id",
        run_id,
        "--format",
        "mermaid",
    )
    # Node n0 is the first node (draft_docs); n1 is review_docs (now queued
    # because draft completed); n2 is apply_docs (still blocked on review).
    assert mermaid.startswith("flowchart TD\n")
    assert "classDef state-completed fill:#c8e6c9" in mermaid
    assert "classDef state-queued fill:#e0e0e0" in mermaid
    assert "classDef state-blocked fill:#fff59d" in mermaid
    assert "class n0 state-completed" in mermaid
    assert "class n1 state-queued" in mermaid
    assert "class n2 state-blocked" in mermaid


def test_run_graph_json_includes_current_state(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path, DOCS_REVIEW_WORKFLOW)
    author = register(tmp_path, run_id, "author", "local")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/docs-review-flow/DOCS_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "local")
    review_packet = claim(tmp_path, reviewer)
    verdict_claimed_review(
        tmp_path,
        reviewer,
        review_packet,
        verdict="accept",
        path="docs/reviews/docs-review-flow/review/DOCS_REVIEW.md",
    )

    payload = data(
        run_cli(
            tmp_path,
            "run",
            "graph",
            "--run-id",
            run_id,
            "--format",
            "json",
        )
    )
    assert payload["run_id"] == run_id
    assert payload["workflow_id"] == "docs-review-flow"
    graph_obj = payload["graph"]
    assert isinstance(graph_obj, dict)
    nodes_by_id = {str(node["job_id"]): node for node in graph_obj["nodes"]}
    for node in nodes_by_id.values():
        assert "current_state" in node
        assert "attempt" in node
    assert nodes_by_id["draft_docs"]["current_state"] == "completed"
    assert nodes_by_id["review_docs"]["current_state"] == "completed"
    review_verdict = nodes_by_id["review_docs"]["latest_verdict"]
    assert isinstance(review_verdict, dict)
    assert review_verdict["verdict"] == "accept"
    apply_node = nodes_by_id["apply_docs"]
    # apply_docs is queued after the review accepts; reviewers do not have
    # write capability so this is the next claimable job.
    assert apply_node["current_state"] in {"queued", "blocked"}
    # Non-review nodes should not carry latest_verdict.
    assert "latest_verdict" not in nodes_by_id["draft_docs"]


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


def test_claim_next_filters_fresh_session_required_in_sql(tmp_path: Path) -> None:
    """The SQL filter must hide fresh-session-required work from any session
    that has already received a packet on this run, even when the message is
    pending and otherwise eligible. Regression for the database-side filter
    in ``claim_next``."""

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
    review_packet = claim(tmp_path, reviewer)
    _, message_id, lease_id = packet_ids(review_packet)
    # Release the review back to pending so the message is otherwise claimable.
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
        "fresh-filter regression",
        "--requeue",
    )

    # Sanity-check the SQL precondition: the released message is pending and
    # the session has a recorded work packet.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        msg_state = conn.execute(
            "SELECT state FROM queue_messages WHERE message_id = ?",
            (message_id,),
        ).fetchone()
        assert msg_state is not None and msg_state[0] == "pending"
        prior_packets = conn.execute(
            "SELECT COUNT(*) FROM work_packets WHERE run_id = ? AND session_id = ?",
            (run_id, reviewer),
        ).fetchone()
        assert prior_packets is not None and prior_packets[0] >= 1
    finally:
        conn.close()

    # The same session must not reclaim a fresh-session-required job it has
    # already touched, even though the message is otherwise eligible.
    no_work = data(run_cli(tmp_path, "claim-next", "--session-id", reviewer))
    assert no_work["status"] == "no_work"


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


def test_override_verdict_accepts_completed_needs_revision_with_findings(
    tmp_path: Path,
) -> None:
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
    packet = claim(tmp_path, codex)
    review_job_id, _message_id, _lease_id = packet_ids(packet)
    verdict_claimed_review(
        tmp_path,
        codex,
        packet,
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )
    ledger = register(tmp_path, run_id, "ledger", "codex")
    assert data(run_cli(tmp_path, "claim-next", "--session-id", ledger))["status"] == "no_work"

    operator = register(tmp_path, run_id, "reviewer", "codex")
    override = data(
        run_cli(
            tmp_path,
            "override-verdict",
            "--session-id",
            operator,
            "--job-id",
            review_job_id,
            "--verdict",
            "accept_with_findings",
            "--rationale",
            "Operator accepts with findings instead of taking the revision loop.",
        )
    )

    assert override["status"] == "overridden"
    assert override["previous_verdict"] == "needs_revision"
    assert override["verdict"] == "accept_with_findings"
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["human_checkpoints"] == []
    assert status["latest_non_accepting_review_verdicts"] == []
    packet = claim(tmp_path, ledger)
    assert packet["job"]["workflow_job_id"] == "findings_ledger"


def test_override_verdict_accepts_already_completed_needs_revision_review(
    tmp_path: Path,
) -> None:
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
    packet = claim(tmp_path, codex)
    review_job_id, _message_id, _lease_id = packet_ids(packet)
    verdict_claimed_review(
        tmp_path,
        codex,
        packet,
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE jobs SET state = 'completed' WHERE job_id = ?",
            (review_job_id,),
        )
        conn.execute(
            "UPDATE blockers SET state = 'resolved', resolved_at = '2026-05-10T00:00:00Z' WHERE job_id = ?",
            (review_job_id,),
        )
        conn.commit()
    finally:
        conn.close()
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )

    operator = register(tmp_path, run_id, "reviewer", "codex")
    data(
        run_cli(
            tmp_path,
            "override-verdict",
            "--session-id",
            operator,
            "--job-id",
            review_job_id,
            "--verdict",
            "accept_with_findings",
            "--rationale",
            "Operator accepts with findings after reading the completed review.",
        )
    )

    ledger = register(tmp_path, run_id, "ledger", "codex")
    packet = claim(tmp_path, ledger)
    assert packet["job"]["workflow_job_id"] == "findings_ledger"


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


def test_evidence_redaction_drops_unknown_fields_by_default(
    tmp_path: Path, monkeypatch: Any
) -> None:
    from striatum import cli as cli_module
    from striatum.db import connect, db_path

    private_marker = "agent prose here that must never escape"
    workflow_path = WORKFLOW
    run_id = prepare_started_run(tmp_path, workflow_path=workflow_path)
    real_snapshot = cli_module.evidence_snapshot

    def patched_snapshot(conn: sqlite3.Connection, *, run_id: str) -> JsonDict:
        payload = real_snapshot(conn, run_id=run_id)
        payload["future_unknown_field"] = private_marker
        # Inject a nested unknown field inside a known list element.
        if payload.get("artifacts"):
            artifacts = payload["artifacts"]
            assert isinstance(artifacts, list)
            for entry in artifacts:
                if isinstance(entry, dict):
                    entry["future_nested_field"] = private_marker
        return payload

    monkeypatch.setattr(cli_module, "evidence_snapshot", patched_snapshot)

    assert db_path(tmp_path).exists()
    with connect(tmp_path) as conn:
        cli_module.evidence_export(
            conn,
            repo=tmp_path,
            run_id=run_id,
            path_text="docs/reviews/rfc-ledger/RUN_EVIDENCE.md",
        )
    evidence = (tmp_path / "docs/reviews/rfc-ledger/RUN_EVIDENCE.md").read_text(encoding="utf-8")
    assert private_marker not in evidence
    assert "future_unknown_field" in evidence
    assert "<redacted-free-text>" in evidence


def test_evidence_redaction_preserves_safe_fields(tmp_path: Path) -> None:
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
        rationale="agent prose that should be redacted",
    )
    data(
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
    evidence = (tmp_path / "docs/reviews/rfc-ledger/RUN_EVIDENCE.md").read_text(encoding="utf-8")
    assert run_id in evidence
    assert "striatum/v1-test" in evidence
    assert "striatum.evidence.v1" in evidence
    assert "needs_revision" in evidence
    # Author identity metadata stays safe.
    assert "reviewer" in evidence
    assert "codex" in evidence
    assert "gpt-5.5" in evidence
    # Job ids and workflow job ids stay safe.
    assert "draft" in evidence
    # Content hash is preserved.
    assert "content_sha256" in evidence
    # Schema version from doctor() stays safe.
    assert "schema_version" in evidence


def test_evidence_redacts_workflow_job_titles(tmp_path: Path) -> None:
    distinctive_title = "DISTINCTIVE_TITLE_project_secret_alpha_12345"
    workflow = example_workflow()
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    for job in jobs:
        assert isinstance(job, dict)
        job["title"] = distinctive_title
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
    data(
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
    evidence = (tmp_path / "docs/reviews/rfc-ledger/RUN_EVIDENCE.md").read_text(encoding="utf-8")
    assert distinctive_title not in evidence


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
    # Verdicts are now grouped by review job. The latest verdict and the
    # attempt count both surface in a single line so the summary stays
    # readable when a review cycles through several attempts.
    assert "`review_codex` (1 attempts): `needs_revision`" in summary
    assert "`finding` `review`: `docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md`" in summary
    # Each artifact carries the structured author byline so a reader can see
    # which role/model produced it without opening the artifact file.
    assert "author: reviewer-codex-gpt-5.5-001" in summary
    assert "`human_checkpoint`" in summary
    # Branch context records what the run was prepared with; without a real
    # git checkout the current branch is None and the recorded branch is
    # surfaced verbatim. With no git current branch detected the line stays
    # short and never claims a MISMATCH.
    assert "Branch: `striatum/v1-test`" in summary
    assert "(MISMATCH)" not in summary
    # Timing block always appears, even when started_at/completed_at are unset.
    assert "## Timing" in summary
    assert "- Created at: `" in summary
    assert "- Started at: `" in summary


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
        "enforcement": "advisory_strict",
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


def test_process_adapter_scrubs_proxy_env_when_network_forbidden(tmp_path: Path) -> None:
    workflow = example_workflow()
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    codex = lanes["codex"]
    assert isinstance(codex, dict)
    codex["constraints"] = {"network": "forbidden", "repo_scope": "local_only"}
    codex["command"] = [
        sys.executable,
        "-c",
        (
            "import os, pathlib, sys; sys.stdin.read(); "
            "scratch = pathlib.Path(os.environ['STRIATUM_SCRATCH_DIR']); "
            "scratch.mkdir(parents=True, exist_ok=True); "
            "(scratch / 'env.txt').write_text("
            "  '\\n'.join("
            "    f'{k}={v}' for k, v in os.environ.items()"
            "    if k in ('HTTP_PROXY','HTTPS_PROXY','ALL_PROXY','http_proxy','https_proxy','all_proxy',"
            "             'STRIATUM_NETWORK_POLICY','STRIATUM_REPO_SCOPE')"
            "  ), encoding='utf-8'"
            ")"
        ),
    ]
    workflow_path = temporary_workflow(tmp_path, workflow)
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    _, _, lease_id = packet_ids(packet)

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["HTTP_PROXY"] = "http://proxy.example:3128"
    env["HTTPS_PROXY"] = "http://proxy.example:3128"
    env["http_proxy"] = "http://proxy.example:3128"
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path),
         "adapter", "run", "--session-id", author, "--lease-id", lease_id, "--json"],
        cwd=tmp_path, env=env, text=True, capture_output=True, check=True,
    )
    result = json.loads(proc.stdout)["data"]
    assert result["exit_code"] == 0
    env_dump = (Path(str(result["scratch_path"])) / "env.txt").read_text(encoding="utf-8")
    # Proxy vars must have been scrubbed from the child's env.
    assert "HTTP_PROXY=" not in env_dump
    assert "HTTPS_PROXY=" not in env_dump
    assert "http_proxy=" not in env_dump
    # Sentinels must be present.
    assert "STRIATUM_NETWORK_POLICY=forbidden" in env_dump
    assert "STRIATUM_REPO_SCOPE=local_only" in env_dump


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
    assert confirmed["mode"] == "records_only"
    assert confirmed["created"] is False


def _git_init_repo(repo: Path, initial_branch: str = "main") -> None:
    """Initialize a git repo with at least one commit so checkout is unambiguous."""
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", initial_branch], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "config", "user.email", "test@example.com"], cwd=repo, check=True, capture_output=True
    )
    subprocess.run(
        ["git", "config", "user.name", "Test"], cwd=repo, check=True, capture_output=True
    )
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "seed", "--no-gpg-sign"],
        cwd=repo,
        check=True,
        capture_output=True,
    )


def _current_branch(repo: Path) -> str:
    result = subprocess.run(
        ["git", "branch", "--show-current"],
        cwd=repo,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def test_branch_confirm_create_runs_git_checkout_b(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    confirmed = data(
        run_cli(
            tmp_path,
            "branch",
            "confirm",
            "--run-id",
            str(prepared["run_id"]),
            "--branch",
            "striatum/created-here",
            "--create",
        )
    )
    assert confirmed["created"] is True
    assert confirmed["mode"] == "create"
    assert confirmed["branch"] == "striatum/created-here"
    assert confirmed["records_only"] is True
    assert _current_branch(tmp_path) == "striatum/created-here"


def test_branch_confirm_create_falls_back_to_checkout_when_branch_exists(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    subprocess.run(
        ["git", "branch", "striatum/already-here"],
        cwd=tmp_path,
        check=True,
        capture_output=True,
    )
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    confirmed = data(
        run_cli(
            tmp_path,
            "branch",
            "confirm",
            "--run-id",
            str(prepared["run_id"]),
            "--branch",
            "striatum/already-here",
            "--create",
        )
    )
    assert confirmed["created"] is False
    assert confirmed["mode"] == "create"
    assert confirmed["branch"] == "striatum/already-here"
    assert _current_branch(tmp_path) == "striatum/already-here"


def test_branch_confirm_use_current_records_actual_branch(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    subprocess.run(
        ["git", "checkout", "-b", "feature/abc"],
        cwd=tmp_path,
        check=True,
        capture_output=True,
    )
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    confirmed = data(
        run_cli(
            tmp_path,
            "branch",
            "confirm",
            "--run-id",
            str(prepared["run_id"]),
            "--branch",
            "feature/abc",
            "--use-current",
        )
    )
    assert confirmed["branch"] == "feature/abc"
    assert confirmed["mode"] == "use_current"
    assert confirmed["created"] is False

    # Conflict path: --use-current with a non-matching --branch must fail with code 8
    # and must NOT update the run state. Use a fresh repo + run so we can re-test.
    second_repo = tmp_path / "second"
    second_repo.mkdir()
    _git_init_repo(second_repo, initial_branch="main")
    subprocess.run(
        ["git", "checkout", "-b", "feature/abc"],
        cwd=second_repo,
        check=True,
        capture_output=True,
    )
    init_repo(second_repo)
    prepared2 = data(run_cli(second_repo, "run", "prepare", "--workflow", str(WORKFLOW)))
    rejected = run_cli(
        second_repo,
        "branch",
        "confirm",
        "--run-id",
        str(prepared2["run_id"]),
        "--branch",
        "something/else",
        "--use-current",
        check=False,
    )
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "use-current" in str(error["message"]) or "current git branch" in str(error["message"])
    # Run state must remain needs_branch_confirmation
    conn = sqlite3.connect(second_repo / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT state FROM runs WHERE run_id = ?", (str(prepared2["run_id"]),)
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] == "needs_branch_confirmation"


def test_branch_confirm_strict_rejects_mismatch(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    rejected = run_cli(
        tmp_path,
        "branch",
        "confirm",
        "--run-id",
        str(prepared["run_id"]),
        "--branch",
        "striatum/foo",
        "--strict",
        check=False,
    )
    assert rejected["returncode"] == 8
    # Run state must NOT have been updated.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT state FROM runs WHERE run_id = ?", (str(prepared["run_id"]),)
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] == "needs_branch_confirmation"

    # Now check out the matching branch and re-run; assert success.
    subprocess.run(
        ["git", "checkout", "-b", "striatum/foo"],
        cwd=tmp_path,
        check=True,
        capture_output=True,
    )
    confirmed = data(
        run_cli(
            tmp_path,
            "branch",
            "confirm",
            "--run-id",
            str(prepared["run_id"]),
            "--branch",
            "striatum/foo",
            "--strict",
        )
    )
    assert confirmed["mode"] == "strict"
    assert confirmed["branch"] == "striatum/foo"
    assert confirmed["warning"] is None
    assert confirmed["records_only"] is True


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

    # The new cycle-soundness check requires the cycle target to feed back into
    # the cycle source through workflow edges. The root reviews don't have an
    # upstream feeder, so insert an "anchor" draft job that does, and target
    # the declared cycles at it.
    workflow["jobs"].insert(
        0,
        {
            "id": "anchor",
            "type": "draft",
            "title": "Anchor draft",
            "role_id": "ledger",
            "lane_id": "codex",
            "objective": "Anchor source for declared_cycle root review cycles.",
            "task_prompt": {"path": "examples/rfc-0014-operational-artifact-home/prompts/findings_ledger.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/reviews/rfc-0014-operational-artifact-home/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {
                    "logical_name": "anchor",
                    "kind": "handoff",
                    "path": "docs/reviews/rfc-0014-operational-artifact-home/RFC_0014_ANCHOR.md",
                    "required": True,
                }
            ],
        },
    )
    workflow["edges"].extend(
        [
            {"from": "anchor", "to": "review_claude", "on": "completed"},
            {"from": "anchor", "to": "review_codex", "on": "completed"},
            {"from": "anchor", "to": "review_gemini", "on": "completed"},
        ]
    )
    for job in workflow["jobs"]:
        if job["id"] in {"review_claude", "review_codex", "review_gemini"}:
            job["needs"] = ["anchor"]
    workflow["cycles"].extend(
        [
            {"from": "review_claude", "to": "anchor", "on_verdict": "needs_revision", "max_iterations": 1},
            {"from": "review_codex", "to": "anchor", "on_verdict": "needs_revision", "max_iterations": 1},
            {"from": "review_gemini", "to": "anchor", "on_verdict": "needs_revision", "max_iterations": 1},
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

def _minimal_validation_workflow() -> dict[str, Any]:
    """Return a tiny valid workflow shape used as a base for validation tests."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-validation",
        "workflow_version": "1",
        "name": "Validation",
        "branch": {"mode": "confirm", "suggested_name": "wf/validation", "allow_dirty": False},
        "coordinator": {"role_id": "author", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {"adapter": "process", "display_model": "X", "command": ["echo"], "capabilities": ["write"]},
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": True},
        "jobs": [],
        "edges": [],
        "cycles": [],
    }


def _validate(workflow_obj: dict[str, Any]) -> None:
    """Invoke the in-process validator (faster than the subprocess CLI)."""
    from striatum.workflow import validate_workflow

    validate_workflow(workflow_obj)


def test_workflow_validation_rejects_cross_job_artifact_path_collision() -> None:
    from striatum.errors import WorkflowError

    workflow = _minimal_validation_workflow()
    workflow["jobs"] = [
        {
            "id": "draft_one",
            "type": "draft",
            "title": "Draft one",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft one",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "draft", "kind": "handoff", "path": "docs/output/SHARED.md", "required": True}
            ],
        },
        {
            "id": "draft_two",
            "type": "draft",
            "title": "Draft two",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft two",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "draft", "kind": "handoff", "path": "docs/output/SHARED.md", "required": True}
            ],
        },
    ]
    with pytest.raises(WorkflowError, match="both declare expected artifact path"):
        _validate(workflow)

    workflow["jobs"][1]["expected_artifacts"][0]["path"] = "docs/output/UNIQUE.md"
    _validate(workflow)


def test_workflow_validation_rejects_overlapping_allowed_and_forbidden_paths() -> None:
    from striatum.errors import WorkflowError

    workflow = _minimal_validation_workflow()
    workflow["jobs"] = [
        {
            "id": "draft",
            "type": "draft",
            "title": "Draft",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": [".striatum/foo"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [],
        },
    ]
    with pytest.raises(WorkflowError, match="is inside forbidden_path"):
        _validate(workflow)

    workflow["jobs"][0]["write_scope"]["allowed_paths"] = ["docs/output/"]
    _validate(workflow)


def test_workflow_validation_rejects_artifact_path_outside_write_scope() -> None:
    from striatum.errors import WorkflowError

    workflow = _minimal_validation_workflow()
    workflow["jobs"] = [
        {
            "id": "draft",
            "type": "draft",
            "title": "Draft",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "out", "kind": "handoff", "path": "docs/elsewhere/OUT.md", "required": True}
            ],
        },
    ]
    with pytest.raises(WorkflowError, match="is not inside any allowed_path"):
        _validate(workflow)

    workflow["jobs"][0]["expected_artifacts"][0]["path"] = "docs/output/OUT.md"
    _validate(workflow)


def test_workflow_validation_rejects_unsound_cycle_target() -> None:
    from striatum.errors import WorkflowError

    workflow = _minimal_validation_workflow()
    workflow["roles"]["reviewer"] = {"definition_path": "roles/reviewer.md"}
    workflow["jobs"] = [
        {
            "id": "draft",
            "type": "draft",
            "title": "Draft",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "draft", "kind": "handoff", "path": "docs/output/DRAFT.md", "required": True}
            ],
        },
        {
            "id": "review",
            "type": "review",
            "title": "Review",
            "role_id": "reviewer",
            "lane_id": "lane_a",
            "objective": "Review",
            "task_prompt": {"path": "prompts/review.md"},
            "write_scope": {
                "mode": "review_only_artifact",
                "repo_write": False,
                "allowed_paths": ["docs/reviews/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "review", "kind": "finding", "path": "docs/reviews/REVIEW.md", "required": True}
            ],
        },
        {
            "id": "apply",
            "type": "draft",
            "title": "Apply",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Apply",
            "task_prompt": {"path": "prompts/apply.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/applied/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "applied", "kind": "handoff", "path": "docs/applied/APPLIED.md", "required": True}
            ],
        },
    ]
    workflow["edges"] = [
        {"from": "draft", "to": "review", "on": "completed"},
        {"from": "review", "to": "apply", "on": "completed"},
    ]
    # Cycle target "apply" is downstream of "review", not upstream — unsound.
    workflow["cycles"] = [
        {"from": "review", "to": "apply", "on_verdict": "needs_revision", "max_iterations": 1}
    ]
    with pytest.raises(WorkflowError, match="unsound"):
        _validate(workflow)

    # Cycle target "draft" is upstream of "review" via an edge — sound.
    workflow["cycles"] = [
        {"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}
    ]
    _validate(workflow)


def test_workflow_validation_rejects_mixed_repo_write_modes_in_parallel_group() -> None:
    from striatum.errors import WorkflowError

    workflow = _minimal_validation_workflow()
    workflow["roles"]["reviewer"] = {"definition_path": "roles/reviewer.md"}
    workflow["jobs"] = [
        {
            "id": "writer_a",
            "type": "draft",
            "title": "Writer A",
            "role_id": "author",
            "lane_id": "lane_a",
            "parallel_group": "g1",
            "objective": "Write",
            "task_prompt": {"path": "prompts/write.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/a/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "a", "kind": "handoff", "path": "docs/a/A.md", "required": True}
            ],
        },
        {
            "id": "review_b",
            "type": "review",
            "title": "Review B",
            "role_id": "reviewer",
            "lane_id": "lane_a",
            "parallel_group": "g1",
            "objective": "Review",
            "task_prompt": {"path": "prompts/review.md"},
            "write_scope": {
                "mode": "review_only_artifact",
                "repo_write": False,
                "allowed_paths": ["docs/reviews/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "review", "kind": "finding", "path": "docs/reviews/B.md", "required": True}
            ],
        },
    ]
    with pytest.raises(WorkflowError, match="mixes repo_write and review-only jobs"):
        _validate(workflow)

    # Putting the review job in a different parallel group resolves the conflict.
    workflow["jobs"][1]["parallel_group"] = "g2"
    _validate(workflow)


def test_workflow_validation_warns_on_deprecated_needs() -> None:
    workflow = _minimal_validation_workflow()
    workflow["jobs"] = [
        {
            "id": "draft",
            "type": "draft",
            "title": "Draft",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft",
            "task_prompt": {"path": "prompts/draft.md"},
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "draft", "kind": "handoff", "path": "docs/output/D.md", "required": True}
            ],
            "needs": [],
        },
    ]
    buffer = io.StringIO()
    with redirect_stderr(buffer):
        _validate(workflow)
    stderr_output = buffer.getvalue()
    assert "deprecated 'needs'" in stderr_output
    assert "draft" in stderr_output

    # Removing 'needs' silences the warning.
    workflow["jobs"][0].pop("needs")
    buffer = io.StringIO()
    with redirect_stderr(buffer):
        _validate(workflow)
    assert buffer.getvalue() == ""


def test_code_change_flow_runs_through_revision_cycle(tmp_path: Path) -> None:
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(CODE_CHANGE_WORKFLOW)))
    assert valid["workflow_id"] == "code-change-flow"
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(CODE_CHANGE_WORKFLOW)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/code-change-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)

    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="src/example/draft.py",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/code-change/REVIEW.md",
    )
    # A new draft attempt should be enqueued via the declared cycle.
    next_author = register(tmp_path, run_id, "author", "codex")
    next_packet = claim(tmp_path, next_author)
    assert next_packet["job"]["workflow_job_id"] == "draft_change"
    assert next_packet["job"]["attempt"] == 2
    next_job_id, next_message_id, next_lease_id = packet_ids(next_packet)
    run_cli(tmp_path, "ack", "--session-id", next_author, "--message-id", next_message_id, "--lease-id", next_lease_id)
    write_artifact(tmp_path, "src/example/draft.py", text="revised draft\n")
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        next_author,
        "--job-id",
        next_job_id,
        "--lease-id",
        next_lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "src/example/draft.py",
    )
    run_cli(tmp_path, "complete", "--session-id", next_author, "--job-id", next_job_id, "--lease-id", next_lease_id)

    next_reviewer = register(tmp_path, run_id, "reviewer", "codex")
    next_review_packet = claim(tmp_path, next_reviewer)
    next_review_job_id, next_review_message_id, next_review_lease_id = packet_ids(next_review_packet)
    run_cli(tmp_path, "ack", "--session-id", next_reviewer, "--message-id", next_review_message_id, "--lease-id", next_review_lease_id)
    write_artifact(tmp_path, "docs/code-change/REVIEW.md", text="accept\n")
    next_review_artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            next_reviewer,
            "--job-id",
            next_review_job_id,
            "--lease-id",
            next_review_lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/code-change/REVIEW.md",
        )
    )
    run_cli(
        tmp_path,
        "verdict",
        "--session-id",
        next_reviewer,
        "--job-id",
        next_review_job_id,
        "--lease-id",
        next_review_lease_id,
        "--verdict",
        "accept",
        "--findings-artifact-id",
        str(next_review_artifact["artifact_id"]),
    )

    applier = register(tmp_path, run_id, "author", "codex")
    apply_packet = claim(tmp_path, applier)
    assert apply_packet["job"]["workflow_job_id"] == "apply_change"
    complete_claimed_job(
        tmp_path,
        applier,
        apply_packet,
        logical_name="applied",
        kind="handoff",
        path="src/example/applied.py",
    )
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "completed"


def test_failed_review_cycle_routes_to_human_checkpoint_after_max_iterations(tmp_path: Path) -> None:
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(FAILED_REVIEW_WORKFLOW)))
    assert valid["workflow_id"] == "failed-review-revision-cycle"
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(FAILED_REVIEW_WORKFLOW)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/failed-review-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)

    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="src/example/draft.py",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    first_verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/failed-review/REVIEW.md",
    )
    assert first_verdict["status"] == "revision_requested"

    next_author = register(tmp_path, run_id, "author", "codex")
    next_packet = claim(tmp_path, next_author)
    next_job_id, next_message_id, next_lease_id = packet_ids(next_packet)
    run_cli(tmp_path, "ack", "--session-id", next_author, "--message-id", next_message_id, "--lease-id", next_lease_id)
    write_artifact(tmp_path, "src/example/draft.py", text="revised draft\n")
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        next_author,
        "--job-id",
        next_job_id,
        "--lease-id",
        next_lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "src/example/draft.py",
    )
    run_cli(tmp_path, "complete", "--session-id", next_author, "--job-id", next_job_id, "--lease-id", next_lease_id)

    next_reviewer = register(tmp_path, run_id, "reviewer", "codex")
    next_review_packet = claim(tmp_path, next_reviewer)
    next_review_job_id, next_review_message_id, next_review_lease_id = packet_ids(next_review_packet)
    run_cli(tmp_path, "ack", "--session-id", next_reviewer, "--message-id", next_review_message_id, "--lease-id", next_review_lease_id)
    write_artifact(tmp_path, "docs/failed-review/REVIEW.md", text="second revision request\n")
    second_review_artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            next_reviewer,
            "--job-id",
            next_review_job_id,
            "--lease-id",
            next_review_lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/failed-review/REVIEW.md",
        )
    )
    second_verdict = data(
        run_cli(
            tmp_path,
            "verdict",
            "--session-id",
            next_reviewer,
            "--job-id",
            next_review_job_id,
            "--lease-id",
            next_review_lease_id,
            "--verdict",
            "needs_revision",
            "--findings-artifact-id",
            str(second_review_artifact["artifact_id"]),
        )
    )
    assert second_verdict["status"] == "waiting_human"
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "running"
    checkpoints = status["human_checkpoints"]
    assert isinstance(checkpoints, list)
    assert len(checkpoints) >= 1
    assert second_verdict["blocker_id"] in {cp["blocker_id"] for cp in checkpoints}


def test_doctor_verbose_includes_problem_records(tmp_path: Path) -> None:
    """`doctor --verbose` augments string problems with structured records."""
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    # Create a packet to capture both a real lease and message id, then snip
    # the lease so the queue message becomes "stale claim", and mark the run
    # completed so the still-active session is "active session on terminal run".
    packet = claim(tmp_path, author)
    _, message_id, lease_id = packet_ids(packet)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'test' WHERE lease_id = ?",
            ("2026-05-07T00:00:00Z", lease_id),
        )
        conn.execute(
            "UPDATE runs SET state = 'completed', completed_at = ? WHERE run_id = ?",
            ("2026-05-07T00:00:00Z", run_id),
        )
        conn.commit()
    finally:
        conn.close()

    # Without --verbose the historical contract still holds: the response is
    # the string list and nothing else.
    plain = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert plain["ok"] is False
    assert isinstance(plain["problems"], list)
    assert "problem_records" not in plain

    verbose = data(run_cli(tmp_path, "doctor", "--run-id", run_id, "--verbose"))
    assert verbose["ok"] is False
    assert isinstance(verbose["problems"], list)
    assert verbose["problems"] == plain["problems"], (
        "verbose mode must not change the string problems list"
    )
    records = verbose["problem_records"]
    assert isinstance(records, list)
    assert len(records) == len(verbose["problems"])
    by_check: dict[str, list[JsonDict]] = {}
    for record in records:
        assert isinstance(record, dict)
        by_check.setdefault(str(record["check"]), []).append(cast(JsonDict, record))
    assert "stale_queue_message_claim" in by_check
    stale = by_check["stale_queue_message_claim"][0]
    assert stale["id"] == message_id
    assert isinstance(stale["context"], dict)
    assert stale["context"]["current_lease_id"] == lease_id
    assert "active_session_on_terminal_run" in by_check
    terminal = by_check["active_session_on_terminal_run"][0]
    assert terminal["id"] == author
    assert isinstance(terminal["context"], dict)
    assert terminal["context"]["run_state"] == "completed"


def test_workflow_init_writes_validating_template(tmp_path: Path) -> None:
    """``workflow init`` produces trees that pass ``workflow validate``."""
    init_repo(tmp_path)
    minimal_dir = tmp_path / "examples" / "starter-minimal"
    review_dir = tmp_path / "examples" / "starter-review"
    code_change_dir = tmp_path / "examples" / "starter-code-change"

    minimal = data(
        run_cli(tmp_path, "workflow", "init", "--style", "minimal", str(minimal_dir))
    )
    assert minimal["status"] == "created"
    assert minimal["style"] == "minimal"
    assert (minimal_dir / "workflow.json").exists()
    assert (minimal_dir / "roles" / "author.md").exists()
    assert (minimal_dir / "prompts" / "draft.md").exists()
    assert not (minimal_dir / "roles" / "reviewer.md").exists()

    review = data(run_cli(tmp_path, "workflow", "init", str(review_dir)))
    assert review["style"] == "review"
    for relative in (
        "workflow.json",
        "roles/author.md",
        "roles/reviewer.md",
        "prompts/draft.md",
        "prompts/review.md",
        "prompts/apply.md",
    ):
        assert (review_dir / relative).exists(), relative

    code_change = data(
        run_cli(
            tmp_path,
            "workflow",
            "init",
            "--style",
            "code-change",
            str(code_change_dir),
        )
    )
    assert code_change["style"] == "code-change"
    code_change_workflow = json.loads(
        (code_change_dir / "workflow.json").read_text(encoding="utf-8")
    )
    cycles = code_change_workflow["cycles"]
    assert isinstance(cycles, list) and len(cycles) == 1
    assert cycles[0]["on_verdict"] == "needs_revision"

    for path in (minimal_dir, review_dir, code_change_dir):
        validated = data(
            run_cli(tmp_path, "workflow", "validate", str(path / "workflow.json"))
        )
        assert validated["valid"] is True

    # Refuses to overwrite an existing path; surface the error envelope so
    # the operator can see why init refused.
    refused = run_cli(
        tmp_path,
        "workflow",
        "init",
        "--style",
        "minimal",
        str(minimal_dir),
        check=False,
    )
    assert refused["returncode"] != 0
    assert "refuses to overwrite" in refused["error"]["message"]


def test_human_checkpoint_flow_records_owner_decision_and_unblocks_downstream(
    tmp_path: Path,
) -> None:
    """The human-checkpoint fixture surfaces an explicit operator checkpoint.

    The decide job's session calls ``block --severity human_checkpoint`` from
    a regular claimed job, status reflects the open human_checkpoint blocker
    and the ``resolve_human_checkpoint`` next action, and the operator records
    the durable decision artifact via ``striatum decision record`` (kind
    ``decision``, no lease required).
    """
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(HUMAN_CHECKPOINT_WORKFLOW)))
    assert valid["workflow_id"] == "human-checkpoint-flow"
    run_id = str(
        data(run_cli(tmp_path, "run", "prepare", "--workflow", str(HUMAN_CHECKPOINT_WORKFLOW)))[
            "run_id"
        ]
    )
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/human-checkpoint-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)

    author = register(tmp_path, run_id, "author", "local")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="analysis",
        kind="handoff",
        path="docs/checkpoints/human-checkpoint-flow/ANALYSIS.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "local")
    review_verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="accept",
        path="docs/checkpoints/human-checkpoint-flow/review/REVIEW.md",
    )
    assert review_verdict["status"] == "completed"
    assert review_verdict["verdict"] == "accept"

    # The decide job is a regular claimable human_checkpoint-typed job. Its
    # session surfaces the explicit operator checkpoint via
    # ``block --severity human_checkpoint``.
    decider = register(tmp_path, run_id, "human_checkpoint", "local")
    decide_packet = claim(tmp_path, decider)
    assert decide_packet["job"]["workflow_job_id"] == "decide"
    decide_job_id, decide_message_id, decide_lease_id = packet_ids(decide_packet)
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        decider,
        "--message-id",
        decide_message_id,
        "--lease-id",
        decide_lease_id,
    )
    blocked = data(
        run_cli(
            tmp_path,
            "block",
            "--session-id",
            decider,
            "--job-id",
            decide_job_id,
            "--lease-id",
            decide_lease_id,
            "--kind",
            "owner_decision",
            "--severity",
            "human_checkpoint",
            "--description",
            "operator must accept or reject the analysis",
        )
    )
    assert blocked["status"] == "blocked"
    blocker_id = blocked["blocker_id"]

    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    checkpoints = status["human_checkpoints"]
    assert isinstance(checkpoints, list)
    assert len(checkpoints) == 1
    checkpoint = checkpoints[0]
    assert checkpoint["blocker_id"] == blocker_id
    assert checkpoint["severity"] == "human_checkpoint"
    assert checkpoint["state"] == "open"
    assert checkpoint["workflow_job_id"] == "decide"
    assert "resolve_human_checkpoint" in status["next_actions"]
    # The decide job should be parked in waiting_human until the operator
    # records a decision.
    assert int(status["jobs"].get("waiting_human", 0)) >= 1

    recorded = data(
        run_cli(
            tmp_path,
            "decision",
            "record",
            "--run-id",
            run_id,
            "--path",
            "docs/checkpoints/human-checkpoint-flow/decisions/DECISION.md",
            "--decision-id",
            "human-checkpoint-flow-001",
            "--outcome",
            "accepted",
            "--title",
            "Operator accepts analysis",
            "--rationale",
            "Operator reviewed the upstream analysis and review artifacts.",
        )
    )
    assert recorded["status"] == "recorded"
    assert recorded["outcome"] == "accepted"
    assert recorded["decision_id"] == "human-checkpoint-flow-001"

    # The decision artifact is recorded as kind ``decision`` and lives at the
    # configured checkpoint path.
    decision_path = (
        tmp_path / "docs" / "checkpoints" / "human-checkpoint-flow" / "decisions" / "DECISION.md"
    )
    assert decision_path.exists()
    decision_text = decision_path.read_text(encoding="utf-8")
    assert "schema_version: striatum.decision.v1" in decision_text
    assert "artifact_kind: decision" in decision_text
    assert "outcome: accepted" in decision_text

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        artifact_row = conn.execute(
            """
            SELECT artifact_kind, job_id, session_id, logical_name, repo_path
            FROM artifacts
            WHERE artifact_id = ?
            """,
            (recorded["artifact_id"],),
        ).fetchone()
        assert artifact_row is not None
        assert artifact_row[0] == "decision"
        # ``decision record`` is run-level; it does not bind to a job/session.
        assert artifact_row[1] is None
        assert artifact_row[2] is None
        assert artifact_row[3] == "human-checkpoint-flow-001"
        assert artifact_row[4] == "docs/checkpoints/human-checkpoint-flow/decisions/DECISION.md"
        decision_event = conn.execute(
            "SELECT event_type FROM events WHERE artifact_id = ?",
            (recorded["artifact_id"],),
        ).fetchone()
        assert decision_event is not None
        assert decision_event[0] == "decision.recorded"
    finally:
        conn.close()

    # The run's terminal state reflects the recorded operator decision: the
    # checkpoint blocker is still the open record explaining why downstream
    # work paused, and the durable decision artifact is now part of the run's
    # provenance. The decide job stays in ``waiting_human`` until an operator
    # explicitly resumes it; that explicit resume flow is RFC 0010 future
    # work, so this fixture asserts the blocker, the recorded decision, and
    # the next-action surface that drives the operator workflow.
    final_status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert final_status["runs"][0]["state"] == "running"
    assert {cp["blocker_id"] for cp in final_status["human_checkpoints"]} == {blocker_id}
    assert "resolve_human_checkpoint" in final_status["next_actions"]


def test_adapter_unavailable_flow_rejects_at_validation(tmp_path: Path) -> None:
    """Workflow validation rejects lanes whose adapter cannot satisfy
    ``required_enforcement``.

    The fixture asks for ``network=enforced`` but the configured ``process``
    adapter only provides ``advisory_strict`` for ``network=forbidden``.
    Lowering the requirement to ``advisory_strict`` makes the same workflow
    pass validation cleanly.
    """
    init_repo(tmp_path)
    rejected = run_cli(
        tmp_path,
        "workflow",
        "validate",
        str(ADAPTER_UNAVAILABLE_WORKFLOW),
        check=False,
    )
    assert rejected["returncode"] == 8
    assert rejected["ok"] is False
    error_message = rejected["error"]["message"]
    assert "requires 'enforced' enforcement" in error_message
    assert "'network'" in error_message

    # Lowering the requirement to ``advisory_strict`` makes the same workflow
    # validate cleanly. We mutate a copy of the fixture into a tmp path to
    # avoid touching the on-disk fixture itself.
    workflow_payload = json.loads(
        ADAPTER_UNAVAILABLE_WORKFLOW.read_text(encoding="utf-8")
    )
    workflow_payload["lanes"]["local"]["required_enforcement"] = {
        "network": "advisory_strict",
    }
    relaxed_path = tmp_path / "adapter-unavailable-relaxed.json"
    relaxed_path.write_text(json.dumps(workflow_payload), encoding="utf-8")
    accepted = data(run_cli(tmp_path, "workflow", "validate", str(relaxed_path)))
    assert accepted["valid"] is True
    assert accepted["workflow_id"] == "adapter-unavailable-flow"


# ----- branch.mode: "auto" (default) ---------------------------------------


def test_run_prepare_auto_mode_creates_branch_and_returns_ready(tmp_path: Path) -> None:
    """When workflow.branch.mode is "auto", `run prepare` creates the branch
    automatically and returns state="ready" — no separate `branch confirm`
    step required.
    """
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    workflow = example_workflow()
    workflow["branch"]["mode"] = "auto"
    workflow["branch"]["suggested_name"] = "striatum/auto-mode-fixture"
    workflow_path = temporary_workflow(tmp_path, workflow)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    assert prepared["state"] == "ready"
    assert prepared["branch"] == "striatum/auto-mode-fixture"
    assert prepared["branch_created"] is True
    assert prepared["branch_mode"] == "auto"
    assert _current_branch(tmp_path) == "striatum/auto-mode-fixture"


def test_run_prepare_default_mode_is_auto(tmp_path: Path) -> None:
    """A workflow that omits `branch.mode` defaults to auto."""
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    workflow = example_workflow()
    workflow["branch"].pop("mode", None)
    workflow["branch"]["suggested_name"] = "striatum/default-mode-fixture"
    workflow_path = temporary_workflow(tmp_path, workflow)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    assert prepared["state"] == "ready"
    assert prepared["branch_mode"] == "auto"
    assert _current_branch(tmp_path) == "striatum/default-mode-fixture"


def test_run_prepare_confirm_mode_still_pauses(tmp_path: Path) -> None:
    """Workflows with `branch.mode: "confirm"` keep the manual gate."""
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    # The `WORKFLOW` fixture (`examples/rfc-ledger-cleanup/workflow.json`)
    # declares `mode: "confirm"`. Use it directly.
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    assert prepared["state"] == "needs_branch_confirmation"
    # No branch was checked out implicitly:
    assert _current_branch(tmp_path) == "main"


def test_workflow_validate_rejects_unknown_branch_mode(tmp_path: Path) -> None:
    init_repo(tmp_path)
    workflow = example_workflow()
    workflow["branch"]["mode"] = "frobnicate"
    workflow_path = temporary_workflow(tmp_path, workflow)
    rejected = run_cli(tmp_path, "workflow", "validate", str(workflow_path), check=False)
    assert rejected["returncode"] == 8
    assert "branch.mode" in str(rejected["error"]["message"])


def test_workflow_validate_rejects_auto_without_suggested_name(tmp_path: Path) -> None:
    init_repo(tmp_path)
    workflow = example_workflow()
    workflow["branch"]["mode"] = "auto"
    workflow["branch"].pop("suggested_name", None)
    workflow_path = temporary_workflow(tmp_path, workflow)
    rejected = run_cli(tmp_path, "workflow", "validate", str(workflow_path), check=False)
    assert rejected["returncode"] == 8
    assert "suggested_name" in str(rejected["error"]["message"])


# ----- README graph example drift guard -----------------------------------


def test_writing_workflows_mermaid_block_matches_code_change_flow_graph() -> None:
    """`docs/WRITING_WORKFLOWS.md` embeds a Mermaid diagram of
    `examples/code-change-flow`.

    The block is hand-pasted, so a job rename, edge change, or new cycle in
    the fixture would silently make the doc stale. This test regenerates
    the Mermaid source via ``workflow_graph_mermaid`` and asserts the doc
    contains the exact rendered block — drift fails the suite, not silently.

    The block lived in `README.md § 2b` until RFC 0017 (D062) moved
    workflow-authoring material into `docs/WRITING_WORKFLOWS.md`.
    """
    from striatum.workflow import load_workflow, workflow_graph_mermaid

    workflow = load_workflow(CODE_CHANGE_WORKFLOW)
    expected = workflow_graph_mermaid(workflow).rstrip("\n")
    target = (ROOT / "docs" / "WRITING_WORKFLOWS.md").read_text(encoding="utf-8")
    fenced = "```mermaid\n" + expected + "\n```"
    assert fenced in target, (
        "docs/WRITING_WORKFLOWS.md Mermaid block has drifted from the live "
        f"`workflow graph examples/code-change-flow/workflow.json` output. "
        f"Update the doc block (or this test) to match.\n\n"
        f"--- expected (between mermaid fences) ---\n{expected}\n"
    )
