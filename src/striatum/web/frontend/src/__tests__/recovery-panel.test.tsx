import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import RecoveryPanel, { __testing } from "../islands/recovery-panel";

let container: HTMLDivElement;
let root: Root | undefined;

beforeEach(() => {
  container = document.createElement("div");
  document.body.appendChild(container);
  Object.assign(navigator, {
    clipboard: {
      writeText: vi.fn(async () => undefined),
    },
  });
});

afterEach(() => {
  if (root) {
    act(() => {
      root?.unmount();
    });
  }
  root = undefined;
  container.remove();
  vi.unstubAllGlobals();
});

async function renderPanel(props: React.ComponentProps<typeof RecoveryPanel>) {
  await act(async () => {
    root = createRoot(container);
    root.render(createElement(RecoveryPanel, props));
  });
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("RecoveryPanel", () => {
  it("builds the dry-run argv and copyable command", () => {
    expect(__testing.autoPublishArgv("run_123")).toEqual([
      "recovery",
      "auto-publish",
      "--run-id",
      "run_123",
      "--dry-run",
    ]);
    expect(__testing.argvToCommand(["recovery", "auto-publish", "--run-id", "run 123"])).toBe(
      "striatum recovery auto-publish --run-id 'run 123'",
    );
  });

  it("renders idle state and data-copy recipes", async () => {
    await renderPanel({
      runId: "run_123",
      recipes: [{ label: "Reconcile", command: "striatum recovery process-reconcile --run-id run_123" }],
    });

    expect(container.textContent).toContain("Preview stale-lease artifact recovery");
    expect(container.textContent).toContain("Run a dry-run preview");
    const copyTargets = Array.from(container.querySelectorAll("[data-copy]")).map((el) =>
      el.getAttribute("data-copy"),
    );
    expect(copyTargets).toContain(
      "striatum recovery auto-publish --run-id run_123 --dry-run",
    );
    expect(copyTargets).toContain(
      "striatum recovery process-reconcile --run-id run_123",
    );
  });

  it("posts /v1/invoke argv and renders would-publish rows", async () => {
    let releaseFetch: (() => void) | undefined;
    const fetchMock = vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: "OK",
      async json() {
        await new Promise<void>((resolve) => {
          releaseFetch = resolve;
        });
        return {
          ok: true,
          data: {
            run_id: "run_123",
            dry_run: true,
            published_count: 1,
            published: [
              {
                workflow_job_id: "build_docs",
                session_id: "session_abc",
                would_publish: [
                  {
                    path: "docs/out.md",
                    kind: "handoff",
                    logical_name: "build_handoff",
                  },
                ],
              },
            ],
            skipped_count: 0,
            skipped: [],
          },
        };
      },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await renderPanel({ runId: "run_123" });
    await act(async () => {
      (container.querySelector("button.secondary-button") as HTMLButtonElement).click();
    });

    expect(container.textContent).toContain("Loading dry-run results");
    releaseFetch?.();
    await flush();

    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/invoke",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          argv: ["recovery", "auto-publish", "--run-id", "run_123", "--dry-run"],
        }),
      }),
    );
    expect(container.textContent).toContain("Would publish");
    expect(container.textContent).toContain("build_docs");
    expect(container.textContent).toContain("docs/out.md");
    expect(container.textContent).toContain("session_abc");
  });

  it("renders service errors without publishing", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: false,
        status: 405,
        statusText: "Method Not Allowed",
        async json() {
          return {
            ok: false,
            error: {
              code: 405,
              message: "command requires --allow-mutations: recovery auto-publish",
            },
          };
        },
      })),
    );

    await renderPanel({ runId: "run_123" });
    await act(async () => {
      (container.querySelector("button.secondary-button") as HTMLButtonElement).click();
    });
    await flush();

    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "command requires --allow-mutations",
    );
  });
});
