import { describe, expect, it } from "vitest";
import { __testing } from "../islands/workflow-chooser";

const { buildSpec, isModifierEnabled } = __testing;

describe("workflow-chooser.buildSpec", () => {
  it("trims lane commands and drops empty entries", () => {
    const spec = buildSpec({
      shape: "review",
      laneSet: "single_agent",
      modifiers: [],
      workflowId: "demo",
      name: "",
      scaffoldRoot: "docs/demo",
      artifactRoot: "docs/demo",
      branchSuggestion: "",
      laneCommands: {
        operator: "  /usr/bin/claude  --print  ",
        unused: "",
      },
    });
    expect(spec.lane_commands?.operator).toEqual([
      "/usr/bin/claude",
      "--print",
    ]);
    expect(spec.lane_commands?.unused).toBeUndefined();
  });

  it("falls back to workflow_id when name is empty", () => {
    const spec = buildSpec({
      shape: "review",
      laneSet: "single_agent",
      modifiers: [],
      workflowId: "demo",
      name: "",
      scaffoldRoot: "docs/demo",
      artifactRoot: "docs/demo",
      branchSuggestion: "",
      laneCommands: {},
    });
    expect(spec.name).toBe("demo");
  });

  it("omits branch_suggestion when blank", () => {
    const spec = buildSpec({
      shape: "review",
      laneSet: "single_agent",
      modifiers: [],
      workflowId: "demo",
      name: "Demo",
      scaffoldRoot: "docs/demo",
      artifactRoot: "docs/demo",
      branchSuggestion: "",
      laneCommands: {},
    });
    expect(spec.branch_suggestion).toBeUndefined();
  });
});

describe("workflow-chooser.isModifierEnabled", () => {
  it("disables a modifier when an incompatible peer is selected", () => {
    const ok = isModifierEnabled(
      { id: "human_checkpoint", incompatible_with: ["autonomous"] },
      ["other"],
    );
    expect(ok).toBe(true);

    const blocked = isModifierEnabled(
      { id: "human_checkpoint", incompatible_with: ["autonomous"] },
      ["autonomous"],
    );
    expect(blocked).toBe(false);
  });

  it("treats modifiers without incompatibility lists as always enabled", () => {
    expect(isModifierEnabled({ id: "evidence_audit" }, ["anything"])).toBe(true);
  });
});
