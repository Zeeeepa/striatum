// RFC 0013 V1 - Striatum local web UI.
// Vanilla ES module; no external dependencies. All data flows through
// RFC 0012 endpoints under /v1/*. Read-only; mutation buttons land in
// step 7 / a future RFC.

const app = document.querySelector("#app");
let activeEventSource = null;

function escapeHTML(s) {
  if (s == null) return "";
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\"", "&quot;")
    .replaceAll("'", "&#39;");
}

// Tiny Markdown helper: paragraphs, headings, lists, bold/italic/code,
// fenced code blocks. HTML is escaped at the input boundary first
// (design-review F2), then a small set of patterns is restored.
function renderMarkdown(src) {
  const escaped = escapeHTML(src || "");
  const lines = escaped.split("\n");
  const out = [];
  let inCode = false;
  let codeBuf = [];
  let listType = null;
  let para = [];
  function flushPara() {
    if (para.length === 0) return;
    let text = para.join(" ");
    text = text.replace(/`([^`]+)`/g, "<code>$1</code>");
    text = text.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    text = text.replace(/_([^_]+)_/g, "<em>$1</em>");
    out.push(`<p>${text}</p>`);
    para = [];
  }
  function closeList() {
    if (listType) {
      out.push(listType === "ul" ? "</ul>" : "</ol>");
      listType = null;
    }
  }
  for (const raw of lines) {
    if (raw.startsWith("```")) {
      flushPara();
      closeList();
      if (inCode) {
        out.push(`<pre>${codeBuf.join("\n")}</pre>`);
        codeBuf = [];
        inCode = false;
      } else {
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      codeBuf.push(raw);
      continue;
    }
    if (raw.match(/^### /)) {
      flushPara();
      closeList();
      out.push(`<h3>${raw.slice(4)}</h3>`);
      continue;
    }
    if (raw.match(/^## /)) {
      flushPara();
      closeList();
      out.push(`<h2>${raw.slice(3)}</h2>`);
      continue;
    }
    if (raw.match(/^# /)) {
      flushPara();
      closeList();
      out.push(`<h1>${raw.slice(2)}</h1>`);
      continue;
    }
    if (raw.match(/^[-*] /)) {
      flushPara();
      if (listType !== "ul") {
        closeList();
        out.push("<ul>");
        listType = "ul";
      }
      out.push(`<li>${raw.slice(2)}</li>`);
      continue;
    }
    if (raw.match(/^\d+\. /)) {
      flushPara();
      if (listType !== "ol") {
        closeList();
        out.push("<ol>");
        listType = "ol";
      }
      const m = raw.match(/^\d+\. (.*)$/);
      out.push(`<li>${m[1]}</li>`);
      continue;
    }
    if (raw.trim() === "") {
      flushPara();
      closeList();
      continue;
    }
    para.push(raw);
  }
  if (inCode) out.push(`<pre>${codeBuf.join("\n")}</pre>`);
  flushPara();
  closeList();
  return out.join("\n");
}

async function fetchJson(path) {
  const res = await fetch(path);
  const body = await res.json().catch(() => ({ ok: false, error: { code: res.status, message: res.statusText } }));
  return body;
}

function badge(state) {
  return `<span class="badge badge-${state}">${escapeHTML(state)}</span>`;
}

function chip(value, prefix = "") {
  return `<span class="chip ${prefix}-${value}">${escapeHTML(value)}</span>`;
}

function closeEventSource() {
  if (activeEventSource) {
    activeEventSource.close();
    activeEventSource = null;
  }
}

// --- Views ----------------------------------------------------------

async function renderRunList() {
  app.innerHTML = "<h1>Runs</h1><div id=\"run-list\">Loading...</div>";
  const result = await fetchJson("/v1/runs");
  if (!result.ok) {
    document.querySelector("#run-list").innerHTML = `<div class="error">Failed to load runs: ${escapeHTML(result.error?.message || "")}</div>`;
    return;
  }
  const runs = (result.data?.runs || []);
  if (runs.length === 0) {
    document.querySelector("#run-list").innerHTML = `<p class="empty">No runs yet.</p>`;
    return;
  }
  let html = "<table><thead><tr><th>Run</th><th>State</th><th>Branch</th></tr></thead><tbody>";
  for (const r of runs) {
    html += `<tr>
      <td><a href="#/runs/${escapeHTML(r.run_id)}">${escapeHTML(r.run_id)}</a></td>
      <td>${badge(r.state)}</td>
      <td>${escapeHTML(r.branch_name || "")}</td>
    </tr>`;
  }
  html += "</tbody></table>";
  document.querySelector("#run-list").innerHTML = html;
}

async function renderRunDetail(runId) {
  app.innerHTML = `<h1>Run <code>${escapeHTML(runId)}</code></h1>
    <div class="layout-2col">
      <section><h2>Jobs</h2><div id="jobs">Loading...</div></section>
      <section><h2>Events <span class="muted">(live)</span></h2><div id="events" class="event-log"></div></section>
    </div>`;
  const result = await fetchJson(`/v1/runs/${encodeURIComponent(runId)}`);
  if (!result.ok) {
    document.querySelector("#jobs").innerHTML = `<div class="error">${escapeHTML(result.error?.message || "")}</div>`;
    return;
  }
  const data = result.data || {};
  const jobsByState = data.jobs || {};
  const blocked = data.blocked_downstream_jobs || [];
  const claimable = data.claimable_jobs || [];
  let jobsHtml = `<h3>State counts</h3><div>`;
  for (const [st, n] of Object.entries(jobsByState)) {
    jobsHtml += `${badge(st)} ${n} `;
  }
  jobsHtml += `</div>`;
  if (claimable.length > 0) {
    jobsHtml += `<h3>Claimable</h3><ul>`;
    for (const c of claimable) {
      jobsHtml += `<li>${escapeHTML(c.role_id)}/${escapeHTML(c.lane_id)} (${c.count} packets) — <code>${escapeHTML((c.workflow_job_ids || []).join(", "))}</code></li>`;
    }
    jobsHtml += `</ul>`;
  }
  if (blocked.length > 0) {
    jobsHtml += `<h3>Blocked downstream</h3><ul>`;
    for (const b of blocked) {
      jobsHtml += `<li><a href="#/runs/${escapeHTML(runId)}/jobs/${escapeHTML(b.job_id)}">${escapeHTML(b.workflow_job_id)}</a> (${badge(b.state)})</li>`;
    }
    jobsHtml += `</ul>`;
  }
  jobsHtml += `<p><a href="/v1/runs/${encodeURIComponent(runId)}/dashboard" target="_blank">raw dashboard JSON</a></p>`;
  document.querySelector("#jobs").innerHTML = jobsHtml;

  // Live SSE.
  closeEventSource();
  const evtUrl = `/v1/runs/${encodeURIComponent(runId)}/events`;
  const eventsBox = document.querySelector("#events");
  activeEventSource = new EventSource(evtUrl);
  activeEventSource.addEventListener("striatum.event", (ev) => {
    const payload = JSON.parse(ev.data);
    const row = document.createElement("div");
    row.className = "event-row";
    row.innerHTML = `<span class="meta">${escapeHTML(payload.created_at || "")} #${payload.event_id} </span>
      <code>${escapeHTML(payload.type)}</code>
      ${payload.job_id ? `<span class="muted"> job=${escapeHTML(payload.job_id)}</span>` : ""}`;
    eventsBox.prepend(row);
  });
  activeEventSource.addEventListener("striatum.run_terminal", (ev) => {
    const payload = JSON.parse(ev.data);
    const row = document.createElement("div");
    row.className = "event-row";
    row.innerHTML = `<strong>Run terminal: ${badge(payload.state)}</strong>`;
    eventsBox.prepend(row);
    closeEventSource();
  });
  activeEventSource.addEventListener("error", () => {
    closeEventSource();
  });
}

async function renderJobDetail(runId, jobId) {
  app.innerHTML = `<h1><a href="#/runs/${escapeHTML(runId)}">${escapeHTML(runId)}</a> / Job <code>${escapeHTML(jobId)}</code></h1>
    <div id="job-detail">Loading...</div>`;
  const result = await fetchJson(`/v1/runs/${encodeURIComponent(runId)}/why?id=${encodeURIComponent(jobId)}`);
  const target = document.querySelector("#job-detail");
  if (!result.ok) {
    target.innerHTML = `<div class="error">${escapeHTML(result.error?.message || "")}</div>`;
    return;
  }
  const data = result.data || {};
  let html = "";
  if (data.job) {
    const j = data.job;
    html += `<h2>Job</h2><dl class="kv">
      <dt>workflow_job_id</dt><dd><code>${escapeHTML(j.workflow_job_id || "")}</code></dd>
      <dt>state</dt><dd>${badge(j.state || "")}</dd>
      <dt>type</dt><dd><code>${escapeHTML(j.job_type || "")}</code></dd>
      <dt>role/lane</dt><dd><code>${escapeHTML(j.role_id || "")}/${escapeHTML(JSON.parse(j.lane_selector_json || '{}').lane_id || "")}</code></dd>
    </dl>`;
  }
  if (Array.isArray(data.artifacts) && data.artifacts.length > 0) {
    html += `<h2>Artifacts</h2><table><thead><tr><th>kind</th><th>logical_name</th><th>path</th><th>sha256</th></tr></thead><tbody>`;
    for (const a of data.artifacts) {
      html += `<tr>
        <td><code>${escapeHTML(a.artifact_kind)}</code></td>
        <td><a href="#/runs/${escapeHTML(runId)}/artifacts/${escapeHTML(a.artifact_id)}">${escapeHTML(a.logical_name || "")}</a></td>
        <td><code>${escapeHTML(a.repo_path || "")}</code></td>
        <td><code>${escapeHTML((a.sha256 || "").slice(0, 12))}</code></td>
      </tr>`;
    }
    html += "</tbody></table>";
  }
  if (Array.isArray(data.events) && data.events.length > 0) {
    html += `<h2>Events</h2><div class="event-log">`;
    for (const e of data.events) {
      html += `<div class="event-row"><span class="meta">${escapeHTML(e.created_at)} #${e.event_id} </span><code>${escapeHTML(e.event_type)}</code></div>`;
    }
    html += "</div>";
  }
  if (Array.isArray(data.verdicts) && data.verdicts.length > 0) {
    html += `<h2>Verdicts</h2><ul>`;
    for (const v of data.verdicts) {
      html += `<li>${badge(v.verdict)} <span class="muted">${escapeHTML(v.created_at)}</span></li>`;
    }
    html += "</ul>";
  }
  if (Array.isArray(data.blockers) && data.blockers.length > 0) {
    html += `<h2>Blockers</h2><ul>`;
    for (const b of data.blockers) {
      html += `<li>${badge(b.severity)} <code>${escapeHTML(b.blocker_kind)}</code> — ${escapeHTML(b.description)}</li>`;
    }
    html += "</ul>";
  }
  target.innerHTML = html || `<p class="empty">No data found for ${escapeHTML(jobId)}.</p>`;
}

async function renderArtifactView(runId, artifactId) {
  app.innerHTML = `<h1><a href="#/runs/${escapeHTML(runId)}">${escapeHTML(runId)}</a> / Artifact <code>${escapeHTML(artifactId)}</code></h1>
    <div id="artifact">Loading...</div>`;
  const meta = await fetchJson(`/v1/runs/${encodeURIComponent(runId)}/why?id=${encodeURIComponent(artifactId)}`);
  const target = document.querySelector("#artifact");
  if (!meta.ok) {
    target.innerHTML = `<div class="error">${escapeHTML(meta.error?.message || "")}</div>`;
    return;
  }
  const a = (meta.data?.artifact) || meta.data || {};
  const kind = a.artifact_kind || a.kind || "unknown";
  const fm = a.front_matter || {};
  let html = `<div class="card"><dl class="kv">`;
  html += `<dt>kind</dt><dd><code>${escapeHTML(kind)}</code></dd>`;
  html += `<dt>logical_name</dt><dd>${escapeHTML(a.logical_name || "")}</dd>`;
  html += `<dt>path</dt><dd><code>${escapeHTML(a.repo_path || "")}</code></dd>`;
  html += `<dt>sha256</dt><dd><code>${escapeHTML(a.sha256 || "")}</code></dd>`;
  html += `<dt>published_at</dt><dd>${escapeHTML(a.published_at || a.created_at || "")}</dd>`;
  html += `</dl></div>`;
  // Per-kind front matter formatting:
  if (Object.keys(fm).length > 0) {
    html += `<div class="card"><h3>Front matter</h3>`;
    if (kind === "decision") {
      html += `<div>${badge(fm.outcome || "unknown")} ${fm.follow_up_required ? "follow-up required" : ""}</div>`;
      html += `<div class="muted">${escapeHTML(fm.title || "")}</div>`;
    } else if (kind === "finding") {
      html += `<div>${badge(fm.verdict_intent || "unknown")} ${chip(fm.severity || "info", "chip")}</div>`;
      if (Array.isArray(fm.tags)) {
        html += `<div>${fm.tags.map(t => `<span class="chip">${escapeHTML(t)}</span>`).join("")}</div>`;
      }
    } else if (kind === "harness_improvement_proposal") {
      html += `<div><span class="chip">${escapeHTML(fm.target || "unknown")}</span></div>`;
      html += `<dl class="kv">
        <dt>expected_benefit</dt><dd>${escapeHTML(fm.expected_benefit || "")}</dd>
        <dt>risk</dt><dd>${escapeHTML(fm.risk || "")}</dd>
        <dt>rollback</dt><dd>${escapeHTML(fm.rollback || "")}</dd>
      </dl>`;
    } else {
      html += `<pre>${escapeHTML(JSON.stringify(fm, null, 2))}</pre>`;
    }
    html += `</div>`;
  }
  // Body
  html += `<div class="card"><h3>Body</h3><div id="artifact-body"><span class="muted">Loading...</span></div></div>`;
  target.innerHTML = html;
  // Fetch raw bytes for body display.
  try {
    const res = await fetch(`/v1/artifacts/${encodeURIComponent(artifactId)}/raw`);
    if (!res.ok) {
      document.querySelector("#artifact-body").innerHTML = `<span class="error">raw fetch failed: ${res.status}</span>`;
      return;
    }
    const text = await res.text();
    const path = a.repo_path || "";
    let rendered;
    if (path.endsWith(".md") || path.endsWith(".markdown")) {
      rendered = renderMarkdown(text);
    } else if (path.endsWith(".json")) {
      try {
        rendered = `<pre>${escapeHTML(JSON.stringify(JSON.parse(text), null, 2))}</pre>`;
      } catch (e) {
        rendered = `<pre>${escapeHTML(text)}</pre>`;
      }
    } else {
      rendered = `<pre>${escapeHTML(text)}</pre>`;
    }
    document.querySelector("#artifact-body").innerHTML = rendered;
  } catch (e) {
    document.querySelector("#artifact-body").innerHTML = `<span class="error">${escapeHTML(String(e))}</span>`;
  }
}

async function renderDoctor() {
  app.innerHTML = `<h1>Doctor</h1><div id="doctor">Loading...</div>`;
  const result = await fetchJson("/v1/doctor");
  const target = document.querySelector("#doctor");
  if (!result.ok) {
    target.innerHTML = `<div class="error">${escapeHTML(result.error?.message || "")}</div>`;
    return;
  }
  const records = result.data?.problem_records || [];
  if (records.length === 0) {
    target.innerHTML = `<p class="empty">No problems detected.</p>`;
    return;
  }
  let html = "<ul>";
  for (const r of records) {
    html += `<li><code>${escapeHTML(r.check)}</code> — ${escapeHTML(r.id)}: <span class="muted">${escapeHTML(JSON.stringify(r.context))}</span></li>`;
  }
  html += "</ul>";
  target.innerHTML = html;
}

// --- Routing --------------------------------------------------------

function route() {
  closeEventSource();
  const hash = location.hash || "#/";
  const path = hash.replace(/^#/, "");
  if (path === "/" || path === "") {
    renderRunList();
    return;
  }
  if (path === "/doctor") {
    renderDoctor();
    return;
  }
  let m = path.match(/^\/runs\/([^/]+)\/jobs\/([^/]+)$/);
  if (m) {
    renderJobDetail(decodeURIComponent(m[1]), decodeURIComponent(m[2]));
    return;
  }
  m = path.match(/^\/runs\/([^/]+)\/artifacts\/([^/]+)$/);
  if (m) {
    renderArtifactView(decodeURIComponent(m[1]), decodeURIComponent(m[2]));
    return;
  }
  m = path.match(/^\/runs\/([^/]+)$/);
  if (m) {
    renderRunDetail(decodeURIComponent(m[1]));
    return;
  }
  app.innerHTML = `<p class="empty">Unknown route: ${escapeHTML(path)}</p>`;
}

window.addEventListener("hashchange", route);
window.addEventListener("DOMContentLoaded", route);
route();
