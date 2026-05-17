import { describe, expect, it } from "vitest";
import { __testing } from "../islands/workflow-chooser";

const { buildSpec, lintSummaryText, recommendedForText } = __testing;

describe("workflow-chooser.buildSpec", () => {
  it("trims lane commands and drops empty entries", () => {
    const spec = buildSpec({
      shape: "review",
      laneSet: "single_agent",
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
      workflowId: "demo",
      name: "Demo",
      scaffoldRoot: "docs/demo",
      artifactRoot: "docs/demo",
      branchSuggestion: "",
      laneCommands: {},
    });
    expect(spec.branch_suggestion).toBeUndefined();
  });

  it("sends an empty modifiers array regardless of input", () => {
    const spec = buildSpec({
      shape: "review",
      laneSet: "single_agent",
      workflowId: "demo",
      name: "",
      scaffoldRoot: "docs/demo",
      artifactRoot: "docs/demo",
      branchSuggestion: "",
      laneCommands: {},
    });
    expect(spec.modifiers).toEqual([]);
  });
});

describe("workflow-chooser.recommendedForText", () => {
  it("joins array recommended_for entries with semicolons", () => {
    expect(recommendedForText(["a", "b"]).split(";").map((s) => s.trim())).toEqual([
      "a",
      "b",
    ]);
  });

  it("passes through string recommended_for unchanged", () => {
    expect(recommendedForText("plain")).toBe("plain");
  });

  it("returns empty string for undefined", () => {
    expect(recommendedForText(undefined)).toBe("");
  });
});

describe("workflow-chooser.lintSummaryText", () => {
  it("formats lint warning count and coverage level with score", () => {
    expect(
      lintSummaryText({
        warning_count: 2,
        warnings: ["same-family reviewers", "weak coverage"],
        coverage: {
          level: "weak",
          score: 3,
          max_score: 8,
          checks: [],
        },
      }),
    ).toBe("2 lint warnings; coverage weak (3/8)");
  });

  it("uses the singular label for one lint warning", () => {
    expect(
      lintSummaryText({
        warning_count: 1,
        coverage: {
          level: "adequate",
          score: 5,
          max_score: 8,
          checks: [],
        },
      }),
    ).toContain("1 lint warning");
  });

  it("formats compact lint coverage when max score is absent", () => {
    expect(
      lintSummaryText({
        warning_count: 0,
        coverage: {
          level: "strong",
          score: 7,
        },
      }),
    ).toBe("0 lint warnings; coverage strong (score 7)");
  });

  it("returns an empty string when lint is absent", () => {
    expect(lintSummaryText(undefined)).toBe("");
  });
});
