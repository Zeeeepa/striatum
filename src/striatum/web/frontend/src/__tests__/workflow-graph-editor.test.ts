import { describe, expect, it } from "vitest";
import { __testing } from "../islands/workflow-graph-editor";

const {
  jobsToNodes,
  workflowToEdges,
  syncWorkflowEdges,
  syncWorkflowJobs,
  newJobFromBlock,
  PALETTE_BLOCKS,
} = __testing;

describe("workflow-graph-editor.jobsToNodes", () => {
  it("emits one node per job and preserves ids", () => {
    const nodes = jobsToNodes({
      jobs: [
        { id: "draft_1", type: "generic" },
        { id: "review_1", type: "review" },
      ],
    });
    expect(nodes).toHaveLength(2);
    expect(nodes.map((n) => n.id)).toEqual(["draft_1", "review_1"]);
  });

  it("returns no nodes for an empty workflow", () => {
    expect(jobsToNodes({})).toEqual([]);
  });
});

describe("workflow-graph-editor.workflowToEdges", () => {
  it("renders edges and cycles separately", () => {
    const edges = workflowToEdges({
      edges: [{ from: "a", to: "b", on: "completed" }],
      cycles: [{ from: "b", to: "a", on_verdict: "needs_revision", max_iterations: 2 }],
    });
    expect(edges).toHaveLength(2);
    const cycle = edges.find((e) => e.className === "cycle-edge");
    expect(cycle).toBeTruthy();
    expect(cycle?.label).toContain("needs_revision");
  });
});

describe("workflow-graph-editor.syncWorkflowEdges", () => {
  it("splits cycle edges back out into the cycles array", () => {
    const next = syncWorkflowEdges(
      { edges: [], cycles: [] },
      [
        { id: "1", source: "a", target: "b", data: { on: "completed" } },
        {
          id: "2",
          source: "b",
          target: "a",
          data: { isCycle: true, on_verdict: "needs_revision", max_iterations: 3 },
        },
      ] as never,
    );
    expect(next.edges).toEqual([{ from: "a", to: "b", on: "completed" }]);
    expect(next.cycles).toEqual([
      { from: "b", to: "a", on_verdict: "needs_revision", max_iterations: 3 },
    ]);
  });

  it("defaults the verdict and iteration count when missing", () => {
    const next = syncWorkflowEdges(
      { edges: [] },
      [
        { id: "1", source: "a", target: "b", data: { isCycle: true } },
      ] as never,
    );
    expect(next.cycles?.[0]).toMatchObject({
      on_verdict: "needs_revision",
      max_iterations: 1,
    });
  });
});

describe("workflow-graph-editor.syncWorkflowJobs", () => {
  it("returns a new object with only jobs replaced", () => {
    const prev = { workflow_id: "demo", jobs: [{ id: "a" }] };
    const next = syncWorkflowJobs(prev, [{ id: "b" }]);
    expect(next.workflow_id).toBe("demo");
    expect(next.jobs).toEqual([{ id: "b" }]);
    expect(next).not.toBe(prev);
  });
});

describe("workflow-graph-editor.newJobFromBlock", () => {
  it("picks an unused id with a numeric suffix", () => {
    const job = newJobFromBlock(
      { kind: "review", jobType: "review" },
      [{ id: "review_1" } as never, { id: "review_2" } as never],
    );
    expect(job.id).toBe("review_3");
    expect(job.type).toBe("review");
  });

  it("scopes review write_scope to review_only_artifact_scope", () => {
    const job = newJobFromBlock({ kind: "review", jobType: "review" }, []);
    expect(job.write_scope?.mode).toBe("review_only_artifact_scope");
  });

  it("defaults non-review jobs to repo_write", () => {
    const job = newJobFromBlock({ kind: "implementation", jobType: "build" }, []);
    expect(job.write_scope?.mode).toBe("repo_write");
  });
});

describe("workflow-graph-editor.PALETTE_BLOCKS", () => {
  it("matches the closed RFC 0034 §5 block vocabulary", () => {
    const kinds = PALETTE_BLOCKS.map((b) => b.kind).sort();
    expect(kinds).toEqual(
      [
        "draft",
        "evidence_audit",
        "final_review",
        "human_checkpoint",
        "implementation",
        "review",
        "support_ledger",
        "synthesis",
        "test",
      ].sort(),
    );
  });
});
