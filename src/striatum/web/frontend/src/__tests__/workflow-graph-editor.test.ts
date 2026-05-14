import { describe, expect, it } from "vitest";
import { __testing } from "../islands/workflow-graph-editor";

const {
  jobsToNodes,
  workflowToEdges,
  syncWorkflowEdges,
  syncWorkflowJobs,
  newJobFromBlock,
  jobNodeLabel,
  PALETTE_BLOCKS,
  buildPhaseLayout,
  hasExplicitPhases,
  jobPhaseId,
  phaseIdForY,
  PHASE_BAND_HEIGHT,
  PHASE_NODE_TOP,
  PHASE_COLUMN_WIDTH,
  PHASE_ROW_HEIGHT,
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

  it("renders require_attested_lane in the node body when set", () => {
    const nodes = jobsToNodes({
      jobs: [{ id: "review_1", type: "review", require_attested_lane: true }],
    });
    expect(nodes[0].data.label).toContain("require_attested_lane=true");
  });
});

describe("workflow-graph-editor.jobNodeLabel", () => {
  it("keeps the attested-lane marker out of ordinary nodes", () => {
    expect(jobNodeLabel({ id: "build_1", type: "build" })).toBe(
      "build_1\nbuild",
    );
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

  it("preserves require_attested_lane when replacing jobs for save", () => {
    const next = syncWorkflowJobs(
      { workflow_id: "demo", jobs: [] },
      [{ id: "review_1", type: "review", require_attested_lane: true }],
    );
    expect(next.jobs).toEqual([
      { id: "review_1", type: "review", require_attested_lane: true },
    ]);
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

// ---------------------------------------------------------------------------
// RFC 0045 v1.1 — multi-phase rendering
// ---------------------------------------------------------------------------

const TWO_PHASE_WORKFLOW = {
  schema_version: "striatum.workflow.v1.1",
  phases: [
    { id: "phase_1_design", title: "Design", synthesis_job_id: "synth_1" },
    {
      id: "phase_2_build",
      title: "Build",
      synthesis_job_id: "consolidate_2",
    },
  ],
  jobs: [
    { id: "design_a", phase: "phase_1_design", parallel_group: "design" },
    { id: "design_b", phase: "phase_1_design", parallel_group: "design" },
    { id: "synth_1", phase: "phase_1_design", type: "phase_synthesis" },
    { id: "build_a", phase: "phase_2_build", parallel_group: "build" },
    { id: "consolidate_2", phase: "phase_2_build", type: "phase_synthesis" },
  ],
  edges: [
    { from: "design_a", to: "synth_1", on: "completed" },
    { from: "design_b", to: "synth_1", on: "completed" },
    // cross-phase: phase_1 synthesis gates phase_2 work
    { from: "synth_1", to: "build_a", on: "completed" },
    { from: "build_a", to: "consolidate_2", on: "completed" },
  ],
  cycles: [
    {
      from: "consolidate_2",
      to: "build_a",
      on_verdict: "needs_revision",
      max_iterations: 2,
    },
  ],
};

describe("workflow-graph-editor.hasExplicitPhases", () => {
  it("returns false when phases is absent or empty", () => {
    expect(hasExplicitPhases({})).toBe(false);
    expect(hasExplicitPhases({ phases: [] })).toBe(false);
  });

  it("returns true when phases has at least one entry", () => {
    expect(hasExplicitPhases({ phases: [{ id: "p1" }] })).toBe(true);
  });
});

describe("workflow-graph-editor.jobPhaseId", () => {
  it("prefers phase over phase_id but accepts either", () => {
    expect(jobPhaseId({ id: "j", phase: "p_a" })).toBe("p_a");
    expect(jobPhaseId({ id: "j", phase_id: "p_b" })).toBe("p_b");
    expect(jobPhaseId({ id: "j", phase: "p_a", phase_id: "p_b" })).toBe("p_a");
    expect(jobPhaseId({ id: "j" })).toBe("");
  });
});

describe("workflow-graph-editor.jobsToNodes (v1 path)", () => {
  it("uses the original square grid when phases is absent", () => {
    const nodes = jobsToNodes({
      jobs: [
        { id: "a", type: "generic" },
        { id: "b", type: "generic" },
        { id: "c", type: "generic" },
        { id: "d", type: "generic" },
      ],
    });
    expect(nodes).toHaveLength(4);
    // cols = ceil(sqrt(4)) = 2 → positions are (0,0), (220,0), (0,140), (220,140)
    expect(nodes.map((n) => n.position)).toEqual([
      { x: 0, y: 0 },
      { x: 220, y: 0 },
      { x: 0, y: 140 },
      { x: 220, y: 140 },
    ]);
  });
});

describe("workflow-graph-editor.jobsToNodes (v1.1 phase bucketing)", () => {
  it("buckets jobs into bands by phase and groups by parallel_group", () => {
    const nodes = jobsToNodes(TWO_PHASE_WORKFLOW);
    expect(nodes).toHaveLength(5);
    const byId = Object.fromEntries(nodes.map((n) => [n.id, n.position]));
    // Phase 1 (index 0) → bandTop=0, nodes Y is PHASE_NODE_TOP + rowIndex*ROW
    expect(byId.design_a.y).toBe(PHASE_NODE_TOP);
    expect(byId.design_b.y).toBe(PHASE_NODE_TOP + PHASE_ROW_HEIGHT);
    // The "synth_1" job has no parallel_group; it becomes its own solo group
    // so it lands in a different column from the design_* group.
    expect(byId.synth_1.x).toBeGreaterThanOrEqual(PHASE_COLUMN_WIDTH);
    // Phase 2 (index 1) → bandTop = PHASE_BAND_HEIGHT, nodes Y starts there + PHASE_NODE_TOP
    expect(byId.build_a.y).toBe(PHASE_BAND_HEIGHT + PHASE_NODE_TOP);
    expect(byId.consolidate_2.y).toBe(PHASE_BAND_HEIGHT + PHASE_NODE_TOP);
  });

  it("places unknown-phase jobs into the first phase", () => {
    const nodes = jobsToNodes({
      phases: [{ id: "p1" }, { id: "p2" }],
      jobs: [{ id: "orphan", phase: "p_missing" }],
    });
    expect(nodes).toHaveLength(1);
    expect(nodes[0].position.y).toBe(PHASE_NODE_TOP);
  });
});

describe("workflow-graph-editor.workflowToEdges (v1.1 cross-phase tagging)", () => {
  it("tags only cross-phase edges with the cross-phase-edge class + style", () => {
    const edges = workflowToEdges(TWO_PHASE_WORKFLOW);
    const intra = edges.find((e) => e.source === "design_a" && e.target === "synth_1");
    const cross = edges.find((e) => e.source === "synth_1" && e.target === "build_a");
    expect(intra?.className).toBeUndefined();
    expect(intra?.style).toBeUndefined();
    expect((intra?.data as { crossPhase?: boolean }).crossPhase).toBeUndefined();
    expect(cross?.className).toBe("cross-phase-edge");
    expect(cross?.style).toMatchObject({ stroke: "#000", strokeWidth: 3 });
    expect(cross?.data).toMatchObject({
      crossPhase: true,
      sourcePhase: "phase_1_design",
      targetPhase: "phase_2_build",
    });
  });

  it("does not emit cross-phase styling for v1 workflows", () => {
    const edges = workflowToEdges({
      jobs: [{ id: "a" }, { id: "b" }],
      edges: [{ from: "a", to: "b", on: "completed" }],
    });
    expect(edges[0].className).toBeUndefined();
    expect(edges[0].style).toBeUndefined();
  });

  it("preserves cycle-edge styling and combines classes for cross-phase cycles", () => {
    const workflow = {
      ...TWO_PHASE_WORKFLOW,
      cycles: [
        {
          from: "consolidate_2",
          to: "design_a",
          on_verdict: "needs_revision",
          max_iterations: 1,
        },
      ],
    };
    const edges = workflowToEdges(workflow);
    const cycle = edges.find((e) => e.source === "consolidate_2");
    expect(cycle?.className).toBe("cycle-edge cross-phase-edge");
    expect(cycle?.style).toMatchObject({ stroke: "#000", strokeWidth: 3 });
  });
});

describe("workflow-graph-editor.syncWorkflowEdges (RFC 0045 metadata stripping)", () => {
  it("drops derived crossPhase/sourcePhase/targetPhase keys when serializing", () => {
    const next = syncWorkflowEdges(
      { ...TWO_PHASE_WORKFLOW },
      [
        {
          id: "1",
          source: "synth_1",
          target: "build_a",
          data: {
            on: "completed",
            crossPhase: true,
            sourcePhase: "phase_1_design",
            targetPhase: "phase_2_build",
          },
        },
      ] as never,
    );
    expect(next.edges).toEqual([
      { from: "synth_1", to: "build_a", on: "completed" },
    ]);
    expect(next.edges?.[0]).not.toHaveProperty("crossPhase");
    expect(next.edges?.[0]).not.toHaveProperty("sourcePhase");
    expect(next.edges?.[0]).not.toHaveProperty("targetPhase");
  });
});

describe("workflow-graph-editor.buildPhaseLayout", () => {
  it("indexes jobs by their declared phase", () => {
    const layout = buildPhaseLayout(TWO_PHASE_WORKFLOW);
    expect(layout.hasExplicit).toBe(true);
    expect(layout.phases.map((p) => p.phase.id)).toEqual([
      "phase_1_design",
      "phase_2_build",
    ]);
    expect(layout.byPhaseId.get("phase_1_design")?.jobIds.sort()).toEqual(
      ["design_a", "design_b", "synth_1"].sort(),
    );
    expect(layout.jobPhaseMap.get("build_a")).toBe("phase_2_build");
  });
});

describe("workflow-graph-editor.phaseIdForY (drag-band lookup)", () => {
  it("maps Y to the band index it falls into", () => {
    const layout = buildPhaseLayout(TWO_PHASE_WORKFLOW);
    expect(phaseIdForY(layout, 50)).toBe("phase_1_design");
    expect(phaseIdForY(layout, PHASE_BAND_HEIGHT + 10)).toBe("phase_2_build");
  });

  it("returns null for v1 workflows and for Y outside any band", () => {
    expect(phaseIdForY(buildPhaseLayout({}), 0)).toBeNull();
    const layout = buildPhaseLayout(TWO_PHASE_WORKFLOW);
    expect(phaseIdForY(layout, PHASE_BAND_HEIGHT * 5)).toBeNull();
  });
});

describe("workflow-graph-editor cross-band drag refusal (helper)", () => {
  it("phaseIdForY + jobPhaseMap together flag a cross-band move", () => {
    const layout = buildPhaseLayout(TWO_PHASE_WORKFLOW);
    // build_a lives in phase_2_build at bandTop = PHASE_BAND_HEIGHT.
    const declared = layout.jobPhaseMap.get("build_a");
    // A Y of 10 falls into phase_1_design — that's a cross-band drop.
    const destPhase = phaseIdForY(layout, 10);
    expect(destPhase).toBe("phase_1_design");
    expect(destPhase).not.toBe(declared);
  });
});
