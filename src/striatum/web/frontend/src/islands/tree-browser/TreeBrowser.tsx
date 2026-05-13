/**
 * RFC 0038 V1 — repo file tree browser island.
 *
 * Lazy directory expansion via `GET /v1/repo/tree?path=<rel>`. WAI-ARIA
 * tree semantics (role="tree" / role="treeitem"), roving tabindex,
 * keyboard navigation, fuzzy filter over loaded entries, breadcrumb,
 * polite live region for load failures. File click navigates to the
 * existing single-file viewer at `/view/<path>`.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchRepoTree } from "../../shared/api-client";
import type {
  RepoTreeEntry,
  TreeBrowserProps,
} from "../../shared/types";

const DEFAULT_TREE_URL = "/v1/repo/tree";

interface DirState {
  status: "idle" | "loading" | "loaded" | "error";
  entries: RepoTreeEntry[];
  error?: string;
}

interface FlatRow {
  entry: RepoTreeEntry;
  depth: number;
  expanded: boolean;
  parent: string;
}

function normalizePath(p: string): string {
  let cleaned = (p || "").replace(/^\/+|\/+$/g, "");
  if (cleaned === "" || cleaned === ".") return "";
  return cleaned;
}

function parentOf(p: string): string {
  if (!p) return "";
  const i = p.lastIndexOf("/");
  return i === -1 ? "" : p.slice(0, i);
}

function fuzzyMatch(entry: RepoTreeEntry, needle: string): boolean {
  if (!needle) return true;
  const hay = entry.path.toLowerCase();
  const ndl = needle.toLowerCase();
  let i = 0;
  for (const ch of hay) {
    if (ch === ndl[i]) i += 1;
    if (i === ndl.length) return true;
  }
  return false;
}

function joinUrl(base: string, path: string): string {
  const slash = base.endsWith("/") ? "" : "/";
  return base + slash + path.replace(/^\/+/, "");
}

export default function TreeBrowser(props: TreeBrowserProps) {
  const initialPath = normalizePath(props.rootPath || "");
  const rootLabel = props.rootLabel ?? "/";
  const viewBase = props.viewBase ?? "/view/";
  const treeUrl = props.treeUrl ?? DEFAULT_TREE_URL;

  const [dirs, setDirs] = useState<Record<string, DirState>>({});
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set([initialPath]));
  const [filter, setFilter] = useState("");
  const [activeRow, setActiveRow] = useState<string>(initialPath);
  const liveRegionRef = useRef<HTMLDivElement | null>(null);

  const loadDir = useCallback(
    async (path: string) => {
      setDirs((prev) => {
        if (prev[path] && prev[path].status === "loaded") return prev;
        return { ...prev, [path]: { status: "loading", entries: [] } };
      });
      const result = await fetchRepoTree(path, treeUrl);
      if (!result.ok) {
        setDirs((prev) => ({
          ...prev,
          [path]: { status: "error", entries: [], error: result.error.message },
        }));
        if (liveRegionRef.current) {
          liveRegionRef.current.textContent = `Failed to load ${path || "/"}: ${result.error.message}`;
        }
        return;
      }
      setDirs((prev) => ({
        ...prev,
        [path]: { status: "loaded", entries: result.data.entries },
      }));
      if (liveRegionRef.current) {
        const count = result.data.entries.length;
        liveRegionRef.current.textContent = `Loaded ${count} ${count === 1 ? "entry" : "entries"} for ${path || "root"}.`;
      }
    },
    [treeUrl],
  );

  useEffect(() => {
    void loadDir(initialPath);
  }, [initialPath, loadDir]);

  const toggleExpand = useCallback(
    (path: string) => {
      setExpanded((prev) => {
        const next = new Set(prev);
        if (next.has(path)) {
          next.delete(path);
        } else {
          next.add(path);
          if (!dirs[path] || dirs[path].status === "idle") {
            void loadDir(path);
          }
        }
        return next;
      });
    },
    [dirs, loadDir],
  );

  const breadcrumb = useMemo(() => {
    const parts = initialPath ? initialPath.split("/") : [];
    const segments: Array<{ label: string; path: string }> = [
      { label: rootLabel, path: "" },
    ];
    let acc = "";
    for (const part of parts) {
      acc = acc ? `${acc}/${part}` : part;
      segments.push({ label: part, path: acc });
    }
    return segments;
  }, [initialPath, rootLabel]);

  const flatRows = useMemo<FlatRow[]>(() => {
    const out: FlatRow[] = [];
    const walk = (path: string, depth: number) => {
      const dir = dirs[path];
      if (!dir || dir.status !== "loaded") return;
      const sorted = [...dir.entries].sort((a, b) => {
        if (a.kind !== b.kind) return a.kind === "dir" ? -1 : 1;
        return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      });
      for (const entry of sorted) {
        if (filter && !fuzzyMatch(entry, filter)) continue;
        const isExpanded = expanded.has(entry.path);
        out.push({ entry, depth, expanded: isExpanded, parent: path });
        if (entry.kind === "dir" && isExpanded) {
          walk(entry.path, depth + 1);
        }
      }
    };
    walk(initialPath, 0);
    return out;
  }, [dirs, expanded, filter, initialPath]);

  const navigateToFile = useCallback(
    (entry: RepoTreeEntry) => {
      window.location.href = joinUrl(viewBase, entry.path);
    },
    [viewBase],
  );

  const handleRowClick = useCallback(
    (row: FlatRow) => {
      setActiveRow(row.entry.path);
      if (row.entry.kind === "dir") {
        toggleExpand(row.entry.path);
      } else {
        navigateToFile(row.entry);
      }
    },
    [navigateToFile, toggleExpand],
  );

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>, row: FlatRow, index: number) => {
      const focusRow = (idx: number) => {
        const next = flatRows[idx];
        if (!next) return;
        setActiveRow(next.entry.path);
        const sel = `[data-row-path="${CSS.escape(next.entry.path)}"]`;
        const el = document.querySelector<HTMLElement>(sel);
        el?.focus();
      };
      switch (event.key) {
        case "ArrowDown":
          event.preventDefault();
          focusRow(index + 1);
          break;
        case "ArrowUp":
          event.preventDefault();
          focusRow(index - 1);
          break;
        case "ArrowRight":
          if (row.entry.kind === "dir" && !row.expanded) {
            event.preventDefault();
            toggleExpand(row.entry.path);
          }
          break;
        case "ArrowLeft":
          if (row.entry.kind === "dir" && row.expanded) {
            event.preventDefault();
            toggleExpand(row.entry.path);
          }
          break;
        case "Enter":
        case " ":
          event.preventDefault();
          handleRowClick(row);
          break;
        case "Home":
          event.preventDefault();
          focusRow(0);
          break;
        case "End":
          event.preventDefault();
          focusRow(flatRows.length - 1);
          break;
        default:
          break;
      }
    },
    [flatRows, handleRowClick, toggleExpand],
  );

  const rootDir = dirs[initialPath];
  const rootStatus = rootDir?.status ?? "idle";
  const isEmpty = rootStatus === "loaded" && flatRows.length === 0;

  return (
    <div className="island-root tree-browser">
      <p className="tree-browser-help">
        Browse repository files — click a directory to expand, click a file to
        view it with syntax highlighting.
      </p>
      <nav aria-label="Breadcrumb" className="tree-browser-breadcrumb">
        {breadcrumb.map((seg, i) => (
          <span key={seg.path}>
            {i > 0 && <span className="sep">/</span>}
            <a href={joinUrl(viewBase, seg.path)}>{seg.label || rootLabel}</a>
          </span>
        ))}
      </nav>
      <div className="island-toolbar">
        <input
          type="search"
          placeholder="Filter loaded entries…"
          aria-label="Filter loaded tree entries"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
      </div>
      <div
        ref={liveRegionRef}
        aria-live="polite"
        aria-atomic="true"
        style={{
          position: "absolute",
          width: 1,
          height: 1,
          overflow: "hidden",
          clip: "rect(0 0 0 0)",
        }}
      />
      {rootStatus === "loading" && (
        <div className="island-loading" role="status">
          Loading…
        </div>
      )}
      {rootStatus === "error" && (
        <div className="island-error" role="alert">
          Failed to load tree: {rootDir?.error}
        </div>
      )}
      {isEmpty && filter === "" && (
        <div className="tree-empty">This directory has no entries.</div>
      )}
      {isEmpty && filter !== "" && (
        <div className="tree-empty">No loaded entries match “{filter}”.</div>
      )}
      <ul role="tree" aria-label="Repository file tree" className="tree-list">
        {flatRows.map((row, index) => {
          const dir = dirs[row.entry.path];
          const isLoading = dir?.status === "loading";
          const hasError = dir?.status === "error";
          return (
            <li
              key={row.entry.path}
              role="treeitem"
              aria-expanded={row.entry.kind === "dir" ? row.expanded : undefined}
              aria-level={row.depth + 1}
              aria-selected={activeRow === row.entry.path}
            >
              <div
                className={`tree-row tree-row-${row.entry.kind}`}
                tabIndex={activeRow === row.entry.path ? 0 : -1}
                data-row-path={row.entry.path}
                onClick={() => handleRowClick(row)}
                onKeyDown={(event) => handleKeyDown(event, row, index)}
                style={{ paddingLeft: `${0.5 + row.depth * 1.25}rem` }}
              >
                <span className="tree-twisty" aria-hidden="true">
                  {row.entry.kind === "dir" ? (row.expanded ? "▾" : "▸") : ""}
                </span>
                <span className="tree-icon" aria-hidden="true">
                  {row.entry.kind === "dir" ? "📁" : "📄"}
                </span>
                <span>{row.entry.name}</span>
              </div>
              {row.entry.kind === "dir" && row.expanded && isLoading && (
                <div className="tree-loading">Loading…</div>
              )}
              {row.entry.kind === "dir" && row.expanded && hasError && (
                <div className="island-error" role="alert">
                  Failed to load {row.entry.path}: {dir?.error}{" "}
                  <button
                    type="button"
                    className="secondary-button compact-button"
                    onClick={() => loadDir(row.entry.path)}
                  >
                    Retry
                  </button>
                </div>
              )}
              {row.entry.kind === "dir" &&
                row.expanded &&
                dir?.status === "loaded" &&
                dir.entries.length === 0 && (
                  <div
                    className="tree-empty"
                    style={{ paddingLeft: `${0.5 + (row.depth + 1) * 1.25}rem` }}
                  >
                    This directory has no entries.
                  </div>
                )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

// Internal helpers exported for tests.
export const __testing = { normalizePath, parentOf, fuzzyMatch, joinUrl };
