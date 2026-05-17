/**
 * RFC 0038 V1.5 F2 regression — chooser must consume the server's literal
 * `{ ok: true, data: { templates: [...] } }` shape on first render and
 * render at least one template card without errors.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { createElement } from "react";
import WorkflowChooser from "../islands/workflow-chooser";

const TEMPLATES_ENVELOPE = {
  ok: true,
  data: {
    templates: [
      {
        template_id: "review",
        kind: "shape",
        display_name: "Review",
        summary: "Author-then-review workflow",
        recommended_for: ["author/review cycles"],
        default_lane_sets: ["single_agent"],
      },
      {
        template_id: "multi_phase",
        kind: "shape",
        display_name: "Multi-phase",
        summary: "Phase-scoped tracks",
        recommended_for: "large ordered work",
        default_lane_sets: ["author_reviewer"],
      },
      {
        template_id: "single_agent",
        kind: "lane_set",
        display_name: "Single agent",
        summary: "One lane",
        recommended_for: "single agent",
      },
      {
        template_id: "author_reviewer",
        kind: "lane_set",
        display_name: "Author + reviewer",
        summary: "Two lanes",
        recommended_for: "two agents",
      },
    ],
  },
};

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: "OK",
      async json() {
        return TEMPLATES_ENVELOPE;
      },
    })),
  );
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container.remove();
  vi.unstubAllGlobals();
});

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

function setInputValue(input: HTMLInputElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    "value",
  )?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("WorkflowChooser fetch contract", () => {
  it("reads templates from {ok:true, data:{templates}} and renders shape cards", async () => {
    await act(async () => {
      root = createRoot(container);
      root.render(
        createElement(WorkflowChooser, {
          allowMutations: false,
          templatesUrl: "/workflow-templates",
        }),
      );
    });
    await flush();
    expect(container.textContent).toContain("Review");
    expect(container.textContent).toContain("Multi-phase");
    expect(container.textContent).not.toContain("returned no shapes");
    expect(container.textContent).not.toContain("Loading workflow templates");
    expect(container.querySelectorAll('[role="radio"]').length).toBeGreaterThanOrEqual(2);
  });

  it("surfaces a clear error when templates is missing", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        status: 200,
        statusText: "OK",
        async json() {
          return { ok: false, error: { code: "x", message: "boom" } };
        },
      })),
    );
    await act(async () => {
      root = createRoot(container);
      root.render(
        createElement(WorkflowChooser, {
          allowMutations: false,
          templatesUrl: "/workflow-templates",
        }),
      );
    });
    await flush();
    expect(container.textContent).toContain("Failed to load workflow templates");
  });

  it("renders generated lint separately from generator warnings", async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        return {
          ok: true,
          status: 200,
          statusText: "OK",
          async json() {
            return {
              ok: true,
              data: {
                workflow: { workflow_id: "demo" },
                files: [{ path: "docs/demo/workflow.json", bytes: 42 }],
                warnings: ["generator chose default branch"],
                lint: {
                  warning_count: 2,
                  coverage: {
                    level: "weak",
                    score: 3,
                  },
                },
              },
            };
          },
        };
      }
      return {
        ok: true,
        status: 200,
        statusText: "OK",
        async json() {
          return TEMPLATES_ENVELOPE;
        },
      };
    });
    vi.stubGlobal("fetch", fetchMock);

    await act(async () => {
      root = createRoot(container);
      root.render(
        createElement(WorkflowChooser, {
          allowMutations: false,
          templatesUrl: "/workflow-templates",
        }),
      );
    });
    await flush();

    await act(async () => {
      (container.querySelector('[role="radio"]') as HTMLButtonElement).click();
    });
    await act(async () => {
      (container.querySelector(".primary-button") as HTMLButtonElement).click();
    });
    await act(async () => {
      setInputValue(
        container.querySelector("#chooser-workflow-id") as HTMLInputElement,
        "demo",
      );
    });
    await act(async () => {
      (container.querySelector(".primary-button") as HTMLButtonElement).click();
    });
    await flush();

    expect(container.textContent).toContain("Lint:");
    expect(container.textContent).toContain("2 lint warnings; coverage weak (score 3)");
    expect(container.textContent).toContain("Generator warnings:");
    expect(container.textContent).toContain("generator chose default branch");
  });
});
