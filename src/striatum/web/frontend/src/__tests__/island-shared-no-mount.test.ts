/**
 * RFC 0038 V1.5 F3 regression — the production `island-shared` entry must
 * NOT mount any island. `src/main.ts` is the dev-shell entry that mounts
 * everything; in V1 it was wired to `island-shared` and double-mounted on
 * every page that loaded both the shared bundle and a per-page island.
 *
 * This test loads the production shared entry plus the chooser entry into
 * a JSDOM page that exposes only `#island-workflow-chooser`, and asserts
 * that `createRoot` is called exactly once for that container.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const createRootCalls: HTMLElement[] = [];

vi.mock("react-dom/client", () => ({
  createRoot: (host: HTMLElement) => {
    createRootCalls.push(host);
    return {
      render: () => undefined,
      unmount: () => undefined,
    };
  },
}));

beforeEach(() => {
  createRootCalls.length = 0;
  document.body.innerHTML =
    '<div id="island-workflow-chooser" data-props=\'{"allowMutations": false, "templatesUrl": "/workflow-templates"}\'></div>';
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: "OK",
      async json() {
        return {
          ok: true,
          data: {
            templates: [
              {
                template_id: "review",
                kind: "shape",
                display_name: "Review",
                summary: "x",
                recommended_for: "y",
                default_lane_sets: ["single_agent"],
              },
              {
                template_id: "single_agent",
                kind: "lane_set",
                display_name: "Single agent",
                summary: "x",
                recommended_for: "y",
              },
            ],
          },
        };
      },
    })),
  );
});

afterEach(() => {
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe("island-shared production entry", () => {
  it("mounts nothing on its own", async () => {
    await import("../shared/island-shared-entry");
    expect(createRootCalls).toEqual([]);
  });

  it("does not double-mount when followed by the chooser entry", async () => {
    await import("../shared/island-shared-entry");
    await import("../islands/workflow-chooser/main");
    const choosers = createRootCalls.filter((h) => h.id === "island-workflow-chooser");
    expect(choosers).toHaveLength(1);
  });
});
