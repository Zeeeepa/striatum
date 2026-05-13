/**
 * Generic mount helper for every island.
 *
 * Each Jinja2 page slot looks like:
 *
 *     <div id="island-<name>" data-props='{"...": "..."}' ></div>
 *     <script type="module" src="/static/build/island-<name>.js" defer></script>
 *
 * Large payloads (workflow JSON, file bytes) live in adjacent
 * `<script type="application/json">` or `<script type="text/plain">` tags
 * to avoid oversized escaped attribute payloads.
 */

import { createRoot, type Root } from "react-dom/client";
import type { ReactElement } from "react";

export interface MountOptions<P> {
  /** DOM id of the container element. */
  containerId: string;
  /** Render the React element from the parsed props. */
  render: (props: P) => ReactElement;
  /** Optional fallback props if the container has no `data-props`. */
  defaultProps?: P;
  /** Mount label used in the on-page error panel. */
  label?: string;
}

function readProps<P>(el: HTMLElement, defaults?: P): P {
  const raw = el.getAttribute("data-props");
  if (!raw) {
    if (defaults !== undefined) return defaults;
    return {} as P;
  }
  try {
    return JSON.parse(raw) as P;
  } catch (err) {
    throw new Error(
      `data-props is not valid JSON: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
}

function renderError(host: HTMLElement, label: string, message: string): void {
  const panel = document.createElement("div");
  panel.setAttribute("role", "alert");
  panel.style.padding = "0.75rem 1rem";
  panel.style.border = "1px solid var(--status-failed, #d1242f)";
  panel.style.borderRadius = "6px";
  panel.style.background = "var(--bg-overlay, #21262d)";
  panel.style.color = "var(--fg-primary, #e6edf3)";
  panel.style.fontFamily = "var(--font-mono, monospace)";
  panel.style.fontSize = "0.9em";
  panel.textContent = `${label}: ${message}`;
  host.replaceChildren(panel);
}

export function mount<P>(opts: MountOptions<P>): Root | null {
  const host = document.getElementById(opts.containerId);
  const label = opts.label ?? opts.containerId;
  if (!host) {
    // The page slot was not present; this is normal on pages that do
    // not embed this island. Mount silently.
    return null;
  }
  let props: P;
  try {
    props = readProps<P>(host, opts.defaultProps);
  } catch (err) {
    renderError(host, label, err instanceof Error ? err.message : String(err));
    return null;
  }
  let element: ReactElement;
  try {
    element = opts.render(props);
  } catch (err) {
    renderError(host, label, err instanceof Error ? err.message : String(err));
    return null;
  }
  const root = createRoot(host);
  root.render(element);
  return root;
}

/**
 * Read a same-page `<script type="application/json">` payload and parse it.
 * Returns ``fallback`` when the element is missing or unparseable.
 */
export function readJsonPayload<T>(elementId: string, fallback: T): T {
  const el = document.getElementById(elementId);
  if (!el) return fallback;
  const text = el.textContent ?? "";
  if (!text.trim()) return fallback;
  try {
    return JSON.parse(text) as T;
  } catch {
    return fallback;
  }
}

/**
 * Read a same-page raw text payload (for code viewer source bytes).
 */
export function readTextPayload(elementId: string): string {
  const el = document.getElementById(elementId);
  if (!el) return "";
  return el.textContent ?? "";
}
