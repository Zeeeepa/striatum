/**
 * RFC 0038 V1 — syntax-highlighted code viewer island.
 *
 * Mounts next to a server-rendered `<pre class="code-pre"><code>...</code></pre>`
 * block. The Python view-file route already escapes the file bytes into that
 * block, so the island reads `textContent` (already-decoded plain text) and
 * hydrates a Shiki-rendered version in place of the server-side fallback.
 * Markdown is rendered upstream by the server route and never reaches this
 * island. Files larger than 5 MB skip Shiki and render escaped plain text.
 *
 * Accessibility:
 * - Line numbers are rendered as `aria-hidden`.
 * - Copy button announces "Copied" via `aria-live`.
 * - Raw link opens in a new tab with `rel="noopener"`.
 * - The Wrap toggle controls whitespace; default is no-wrap with internal
 *   horizontal scroll, including for narrow viewports.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CodeViewerProps } from "../../shared/types";

const MAX_HIGHLIGHT_BYTES = 5 * 1024 * 1024;
const COLLAPSE_LINE_THRESHOLD = 500;

const SHIKI_LANG_MAP: Record<string, string> = {
  json: "json",
  py: "python",
  python: "python",
  ts: "typescript",
  typescript: "typescript",
  js: "javascript",
  javascript: "javascript",
  sh: "bash",
  bash: "bash",
  shell: "bash",
  yaml: "yaml",
  yml: "yaml",
  toml: "toml",
  md: "markdown",
  markdown: "markdown",
  sql: "sql",
};

const SUPPORTED_LANGS = [
  "json",
  "python",
  "typescript",
  "javascript",
  "bash",
  "yaml",
  "toml",
  "markdown",
  "sql",
];

interface HighlightState {
  status: "idle" | "loading" | "ready" | "fallback" | "error";
  html?: string;
  language?: string;
  error?: string;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function detectLanguage(language: string | undefined, fileName: string): string {
  if (language) {
    const mapped = SHIKI_LANG_MAP[language.toLowerCase()];
    if (mapped) return mapped;
  }
  const ext = fileName.includes(".")
    ? fileName.split(".").pop()!.toLowerCase()
    : "";
  return SHIKI_LANG_MAP[ext] ?? "plaintext";
}

function plainTextHtml(text: string): string {
  const lines = text.split("\n");
  return lines
    .map((line, i) => {
      const num = String(i + 1).padStart(4, " ");
      return `<div class="line"><span class="line-number" aria-hidden="true">${num}</span><span class="line-content">${
        escapeHtml(line) || "&nbsp;"
      }</span></div>`;
    })
    .join("");
}

function injectLineNumbers(rawHtml: string): string {
  // Shiki's HTML wraps every line in a `<span class="line">…</span>` inside
  // `<pre><code>`. We split on those and decorate with explicit line numbers
  // because the bundled CSS controls the gutter, not Shiki's inline styles.
  const codeMatch = rawHtml.match(/<code>([\s\S]*?)<\/code>/);
  if (!codeMatch) return rawHtml;
  const inner = codeMatch[1];
  const lines = inner.split(/(<span class="line">[\s\S]*?<\/span>)/g).filter(Boolean);
  let lineNo = 0;
  const decorated = lines
    .map((part) => {
      if (!part.startsWith('<span class="line">')) return part;
      lineNo += 1;
      const num = String(lineNo).padStart(4, " ");
      return (
        `<div class="line"><span class="line-number" aria-hidden="true">${num}</span>` +
        `<span class="line-content">${part}</span></div>`
      );
    })
    .join("");
  return decorated;
}

function findAdjacentCodeBlock(selector?: string): HTMLElement | null {
  if (selector) {
    const el = document.querySelector(selector) as HTMLElement | null;
    if (el) return el;
  }
  // Fallback: locate the first `<pre class="code-pre">` element on the page.
  return document.querySelector<HTMLElement>("pre.code-pre code")
    ?? document.querySelector<HTMLElement>("pre.code-pre");
}

export default function CodeViewer(props: CodeViewerProps) {
  const fileName = props.path;
  const rawUrl = props.rawUrl ?? "/view/raw/" + props.path;

  const sourceInfo = useMemo(() => {
    const el = findAdjacentCodeBlock(props.sourceElementSelector);
    const text = el?.textContent ?? "";
    const byteSize =
      typeof TextEncoder !== "undefined"
        ? new TextEncoder().encode(text).length
        : text.length;
    return { text, byteSize, source: el };
  }, [props.sourceElementSelector]);

  const detectedLang = useMemo(
    () => detectLanguage(props.language, fileName),
    [props.language, fileName],
  );
  const lineCount = useMemo(
    () => sourceInfo.text.split("\n").length,
    [sourceInfo.text],
  );

  const [highlight, setHighlight] = useState<HighlightState>({ status: "idle" });
  const [wrap, setWrap] = useState(false);
  const [collapsed, setCollapsed] = useState(lineCount > COLLAPSE_LINE_THRESHOLD);
  const [copied, setCopied] = useState(false);
  const announcementRef = useRef<HTMLDivElement | null>(null);

  // Hide the server-rendered fallback once we've taken over.
  useEffect(() => {
    const fallback = sourceInfo.source?.closest("pre.code-pre");
    if (fallback) {
      (fallback as HTMLElement).style.display = "none";
    }
  }, [sourceInfo.source]);

  useEffect(() => {
    const text = sourceInfo.text;
    if (sourceInfo.byteSize > MAX_HIGHLIGHT_BYTES) {
      setHighlight({
        status: "fallback",
        html: plainTextHtml(text),
        language: "plaintext",
      });
      return;
    }
    if (!SUPPORTED_LANGS.includes(detectedLang)) {
      setHighlight({
        status: "fallback",
        html: plainTextHtml(text),
        language: "plaintext",
      });
      return;
    }
    setHighlight({ status: "loading" });
    let cancelled = false;
    (async () => {
      try {
        const shiki = await import("shiki");
        const highlighter = await shiki.createHighlighter({
          themes: ["github-light", "github-dark"],
          langs: [detectedLang],
        });
        if (cancelled) return;
        const isDark =
          typeof window !== "undefined" &&
          window.matchMedia &&
          window.matchMedia("(prefers-color-scheme: dark)").matches;
        const html = highlighter.codeToHtml(text, {
          lang: detectedLang,
          theme: isDark ? "github-dark" : "github-light",
        });
        setHighlight({
          status: "ready",
          html: injectLineNumbers(html),
          language: detectedLang,
        });
      } catch (err) {
        if (cancelled) return;
        setHighlight({
          status: "fallback",
          html: plainTextHtml(text),
          language: "plaintext",
          error: err instanceof Error ? err.message : String(err),
        });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [detectedLang, sourceInfo.byteSize, sourceInfo.text]);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(sourceInfo.text);
      setCopied(true);
      if (announcementRef.current) {
        announcementRef.current.textContent = "Copied to clipboard.";
      }
      window.setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      if (announcementRef.current) {
        announcementRef.current.textContent =
          "Copy failed: " + (err instanceof Error ? err.message : String(err));
      }
    }
  }, [sourceInfo.text]);

  const bodyClasses = ["code-viewer-body"];
  if (wrap) bodyClasses.push("is-wrap");
  if (collapsed) bodyClasses.push("is-collapsed");

  return (
    <div className="island-root code-viewer">
      <div
        className="code-viewer-toolbar"
        role="toolbar"
        aria-label="Code viewer controls"
      >
        <span className="filename" title={fileName}>
          {fileName}
        </span>
        <span className="muted" aria-label="Detected language">
          {highlight.language ?? detectedLang}
        </span>
        <button
          type="button"
          aria-label="Copy file contents"
          onClick={() => void handleCopy()}
        >
          {copied ? "Copied" : "Copy"}
        </button>
        <button
          type="button"
          aria-label={wrap ? "Disable wrap" : "Enable wrap"}
          aria-pressed={wrap}
          onClick={() => setWrap((w) => !w)}
        >
          Wrap: {wrap ? "on" : "off"}
        </button>
        <a
          href={rawUrl}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Open raw file in a new tab"
        >
          Raw
        </a>
      </div>
      <div
        ref={announcementRef}
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
      <div className={bodyClasses.join(" ")}>
        {highlight.status === "loading" && (
          <pre className="island-loading">Highlighting…</pre>
        )}
        {(highlight.status === "ready" || highlight.status === "fallback") && (
          <div
            // Shiki output is sanitized — it only emits its own spans and
            // escapes the user content. The fallback path uses our own
            // escapeHtml function before assembling the HTML.
            // eslint-disable-next-line react/no-danger
            dangerouslySetInnerHTML={{ __html: highlight.html ?? "" }}
          />
        )}
        {highlight.status === "error" && (
          <pre className="island-error">{highlight.error}</pre>
        )}
      </div>
      {lineCount > COLLAPSE_LINE_THRESHOLD && (
        <div className="code-viewer-collapsed-banner">
          <span>
            File has {lineCount} lines.
            {collapsed ? " Showing the first portion only." : " Showing in full."}
          </span>
          <button
            type="button"
            className="secondary-button compact-button"
            onClick={() => setCollapsed((c) => !c)}
          >
            {collapsed ? "Expand" : "Collapse"}
          </button>
        </div>
      )}
    </div>
  );
}

export const __testing = {
  detectLanguage,
  escapeHtml,
  injectLineNumbers,
  plainTextHtml,
  SHIKI_LANG_MAP,
  SUPPORTED_LANGS,
  MAX_HIGHLIGHT_BYTES,
  COLLAPSE_LINE_THRESHOLD,
};
