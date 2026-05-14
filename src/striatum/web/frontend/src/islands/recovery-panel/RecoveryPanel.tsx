import { useMemo, useState } from "react";

type PanelStatus = "idle" | "loading" | "ready" | "error";

export interface RecoveryRecipe {
  label?: string;
  argv?: string[];
  command?: string;
}

export interface RecoveryPanelProps {
  runId: string;
  invokeUrl?: string;
  recipes?: RecoveryRecipe[];
}

interface InvokeError {
  code?: string | number;
  message: string;
}

interface InvokeOk<T> {
  ok: true;
  data: T;
}

interface InvokeErr {
  ok: false;
  error: InvokeError;
}

type InvokeResult<T> = InvokeOk<T> | InvokeErr;

interface WouldPublishArtifact {
  path?: unknown;
  kind?: unknown;
  logical_name?: unknown;
}

interface PublishedCandidate {
  workflow_job_id?: unknown;
  session_id?: unknown;
  would_publish?: unknown;
}

interface AutoPublishDryRun {
  run_id?: unknown;
  dry_run?: unknown;
  published_count?: unknown;
  published?: unknown;
  skipped_count?: unknown;
  skipped?: unknown;
}

interface WouldPublishRow {
  workflowJobId: string;
  sessionId: string;
  path: string;
  kind: string;
  logicalName: string;
}

const DEFAULT_INVOKE_URL = "/v1/invoke";

export function autoPublishArgv(runId: string): string[] {
  return ["recovery", "auto-publish", "--run-id", runId, "--dry-run"];
}

export function argvToCommand(argv: string[]): string {
  return ["striatum", ...argv].map(shellQuote).join(" ");
}

function shellQuote(part: string): string {
  if (/^[A-Za-z0-9_./:=@+-]+$/.test(part)) return part;
  return "'" + part.replaceAll("'", "'\"'\"'") + "'";
}

function asText(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function normalizeRows(data: AutoPublishDryRun | undefined): WouldPublishRow[] {
  const published = Array.isArray(data?.published) ? data.published : [];
  const rows: WouldPublishRow[] = [];
  for (const item of published) {
    if (!item || typeof item !== "object") continue;
    const candidate = item as PublishedCandidate;
    const artifacts = Array.isArray(candidate.would_publish) ? candidate.would_publish : [];
    for (const artifact of artifacts) {
      if (!artifact || typeof artifact !== "object") continue;
      const row = artifact as WouldPublishArtifact;
      rows.push({
        workflowJobId: asText(candidate.workflow_job_id),
        sessionId: asText(candidate.session_id),
        path: asText(row.path),
        kind: asText(row.kind),
        logicalName: asText(row.logical_name),
      });
    }
  }
  return rows;
}

async function invokeDryRun(
  url: string,
  runId: string,
): Promise<InvokeResult<AutoPublishDryRun>> {
  let response: Response;
  try {
    response = await fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ argv: autoPublishArgv(runId) }),
    });
  } catch (err) {
    return {
      ok: false,
      error: {
        code: "network_error",
        message: err instanceof Error ? err.message : String(err),
      },
    };
  }
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    return {
      ok: false,
      error: {
        code: "non_json_response",
        message: `HTTP ${response.status} ${response.statusText}`,
      },
    };
  }
  if (body && typeof body === "object" && (body as { ok?: unknown }).ok === true) {
    return body as InvokeOk<AutoPublishDryRun>;
  }
  if (body && typeof body === "object" && (body as { error?: unknown }).error) {
    return body as InvokeErr;
  }
  return {
    ok: false,
    error: {
      code: "unexpected_shape",
      message: `HTTP ${response.status}`,
    },
  };
}

function commandForRecipe(recipe: RecoveryRecipe): string {
  if (recipe.command) return recipe.command;
  if (recipe.argv) return argvToCommand(recipe.argv);
  return "";
}

export default function RecoveryPanel(props: RecoveryPanelProps) {
  const [status, setStatus] = useState<PanelStatus>("idle");
  const [result, setResult] = useState<AutoPublishDryRun | undefined>();
  const [error, setError] = useState<string>("");
  const invokeUrl = props.invokeUrl ?? DEFAULT_INVOKE_URL;
  const dryRunArgv = useMemo(() => autoPublishArgv(props.runId), [props.runId]);
  const dryRunCommand = useMemo(() => argvToCommand(dryRunArgv), [dryRunArgv]);
  const rows = useMemo(() => normalizeRows(result), [result]);
  const recipes = useMemo<RecoveryRecipe[]>(() => {
    const explicit = props.recipes ?? [];
    return [{ label: "Auto-publish dry run", argv: dryRunArgv }, ...explicit];
  }, [props.recipes, dryRunArgv]);

  const runDryRun = async () => {
    setStatus("loading");
    setError("");
    setResult(undefined);
    const res = await invokeDryRun(invokeUrl, props.runId);
    if (!res.ok) {
      setStatus("error");
      setError(res.error.message || "Dry run failed");
      return;
    }
    setStatus("ready");
    setResult(res.data);
  };

  if (!props.runId) {
    return (
      <div className="recovery-panel-enhancement" aria-labelledby="recovery-panel-heading">
        <p role="alert" className="island-error">Missing run id.</p>
      </div>
    );
  }

  return (
    <div className="recovery-panel-enhancement" aria-label="Recovery dry-run preview">
      <header className="recovery-panel-header">
        <div>
          <h3>Auto-publish dry run</h3>
          <p className="muted">Preview stale-lease artifact recovery without publishing.</p>
        </div>
        <button
          type="button"
          className="secondary-button"
          onClick={runDryRun}
          disabled={status === "loading"}
        >
          {status === "loading" ? "Checking..." : "Dry run"}
        </button>
      </header>

      <div className="recipe-list" aria-label="Recovery recipes">
        {recipes.map((recipe, index) => {
          const command = commandForRecipe(recipe);
          if (!command) return null;
          return (
            <button
              key={`${command}-${index}`}
              type="button"
              className="copy-command"
              data-copy={command}
              title="Copy command"
            >
              <span>{recipe.label ?? "Recovery command"}</span>
              <code>{command}</code>
            </button>
          );
        })}
      </div>

      {status === "idle" ? (
        <p className="empty-state">Run a dry-run preview before publishing from the CLI.</p>
      ) : null}
      {status === "loading" ? (
        <p className="empty-state" role="status">Loading dry-run results...</p>
      ) : null}
      {status === "error" ? (
        <p className="island-error" role="alert">
          {error || "Dry run failed."}
        </p>
      ) : null}
      {status === "ready" ? (
        <div className="recovery-results">
          {rows.length === 0 ? (
            <p className="empty-state">No artifacts would be published.</p>
          ) : (
            <table>
              <caption>Would publish</caption>
              <thead>
                <tr>
                  <th scope="col">Job</th>
                  <th scope="col">Artifact</th>
                  <th scope="col">Kind</th>
                  <th scope="col">Session</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row, index) => (
                  <tr key={`${row.workflowJobId}-${row.path}-${index}`}>
                    <td>{row.workflowJobId || "-"}</td>
                    <td>
                      <code>{row.path || "-"}</code>
                      {row.logicalName ? <span className="muted"> {row.logicalName}</span> : null}
                    </td>
                    <td>{row.kind || "-"}</td>
                    <td><code>{row.sessionId || "-"}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <p className="muted">
            This panel only runs <code>{dryRunCommand}</code>.
          </p>
        </div>
      ) : null}
    </div>
  );
}

export const __testing = {
  autoPublishArgv,
  argvToCommand,
  normalizeRows,
};
