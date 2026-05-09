// RFC 0024 V1.5: workflow visual builder JS island.
// Reads the initial workflow state from a <script type="application/json">
// tag, renders editable form sections, manages state in memory, and
// POSTs the full state as JSON on save.

(function () {
  var page = document.querySelector(".workflow-edit-page");
  if (!page) return;
  var relPath = page.getAttribute("data-rel-path");
  var dataScript = document.getElementById("workflow-data");
  if (!dataScript) return;

  var state;
  try {
    state = JSON.parse(dataScript.textContent || "{}");
  } catch (err) {
    state = {};
  }

  // RFC 0024 V2: track the on-disk sha256 so we can echo as If-Match.
  var diskSha256 = "";
  var shaScript = document.getElementById("workflow-sha256");
  if (shaScript) {
    try { diskSha256 = JSON.parse(shaScript.textContent || '""'); } catch (err) {}
  }

  var ALLOWED_POSTURES = [
    "neutral", "devils_advocate", "security", "threat_model",
    "latency_performance", "ergonomics_dx", "accessibility",
    "compliance_license", "supply_chain",
  ];
  var JOB_TYPES = ["generic", "review", "synthesis", "build"];
  var EDGE_ON = ["completed", "failed", "blocked"];

  function el(tag, attrs, children) {
    var node = document.createElement(tag);
    if (attrs) {
      for (var key in attrs) {
        if (key === "className") node.className = attrs[key];
        else if (key === "textContent") node.textContent = attrs[key];
        else if (key === "innerHTML") node.innerHTML = attrs[key];
        else if (key.indexOf("on") === 0 && typeof attrs[key] === "function") node.addEventListener(key.substring(2), attrs[key]);
        else if (attrs[key] !== null && attrs[key] !== undefined) node.setAttribute(key, attrs[key]);
      }
    }
    if (children) {
      for (var i = 0; i < children.length; i++) {
        var c = children[i];
        if (c == null) continue;
        if (typeof c === "string") node.appendChild(document.createTextNode(c));
        else node.appendChild(c);
      }
    }
    return node;
  }

  function input(value, oninput, type) {
    return el("input", {
      type: type || "text",
      value: value == null ? "" : String(value),
      oninput: function (e) { oninput(e.target.value); },
    });
  }

  function textarea(value, oninput) {
    return el("textarea", {
      rows: "3",
      oninput: function (e) { oninput(e.target.value); },
      textContent: value == null ? "" : String(value),
    });
  }

  function select(value, options, oninput) {
    var opts = options.map(function (o) {
      return el("option", { value: o, selected: o === value ? "selected" : null }, [o]);
    });
    return el("select", {
      onchange: function (e) { oninput(e.target.value); },
    }, opts);
  }

  function persistDraft() {
    try {
      localStorage.setItem("workflow_edit_" + relPath, JSON.stringify(state));
    } catch (err) { /* localStorage may be unavailable */ }
  }

  function clearDraft() {
    try { localStorage.removeItem("workflow_edit_" + relPath); } catch (err) {}
  }

  function ddWithPath(fieldPath, control) {
    var dd = el("dd", { "data-field-path": fieldPath }, [control]);
    return dd;
  }

  function renderHeader() {
    var container = document.getElementById("edit-header");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Header"]));
    var dl = el("dl", { className: "kv-grid" });
    dl.appendChild(el("dt", {}, ["schema_version"]));
    dl.appendChild(ddWithPath("schema_version", input(state.schema_version, function (v) { state.schema_version = v; persistDraft(); })));
    dl.appendChild(el("dt", {}, ["workflow_id"]));
    dl.appendChild(el("dd", {}, [input(state.workflow_id, function (v) { state.workflow_id = v; persistDraft(); })]));
    dl.appendChild(el("dt", {}, ["workflow_version"]));
    dl.appendChild(el("dd", {}, [input(state.workflow_version, function (v) { state.workflow_version = v; persistDraft(); })]));
    dl.appendChild(el("dt", {}, ["name"]));
    dl.appendChild(el("dd", {}, [input(state.name, function (v) { state.name = v; persistDraft(); })]));
    container.appendChild(dl);
  }

  function renderRoles() {
    var container = document.getElementById("edit-roles");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Roles"]));
    state.roles = state.roles || {};
    var list = el("ul", { className: "edit-list" });
    Object.keys(state.roles).forEach(function (roleId) {
      var role = state.roles[roleId] || {};
      var defPath = role.definition_path || "";
      var idInput = input(roleId, function (v) {
        if (!v || v === roleId) return;
        state.roles[v] = state.roles[roleId];
        delete state.roles[roleId];
        persistDraft(); renderRoles(); renderJobs();
      });
      var pathInput = input(defPath, function (v) {
        state.roles[roleId] = state.roles[roleId] || {};
        state.roles[roleId].definition_path = v;
        persistDraft();
      });
      var rm = el("button", {
        type: "button", className: "remove-btn",
        onclick: function () { delete state.roles[roleId]; persistDraft(); renderRoles(); renderJobs(); },
      }, ["× Remove"]);
      list.appendChild(el("li", {}, [
        el("label", {}, ["id "]), idInput,
        el("label", {}, [" definition_path "]), pathInput,
        rm,
      ]));
    });
    container.appendChild(list);
    container.appendChild(el("button", {
      type: "button", className: "add-btn",
      onclick: function () {
        var id = "role_" + (Object.keys(state.roles).length + 1);
        state.roles[id] = { definition_path: "" };
        persistDraft(); renderRoles(); renderJobs();
      },
    }, ["+ Add role"]));
  }

  function renderLanes() {
    var container = document.getElementById("edit-lanes");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Lanes"]));
    state.lanes = state.lanes || {};
    var list = el("ul", { className: "edit-list" });
    Object.keys(state.lanes).forEach(function (laneId) {
      var lane = state.lanes[laneId] || {};
      var idInput = input(laneId, function (v) {
        if (!v || v === laneId) return;
        state.lanes[v] = state.lanes[laneId];
        delete state.lanes[laneId];
        persistDraft(); renderLanes(); renderJobs();
      });
      var adapterInput = input(lane.adapter, function (v) { state.lanes[laneId].adapter = v; persistDraft(); });
      var modelInput = input(lane.display_model, function (v) { state.lanes[laneId].display_model = v; persistDraft(); });
      var rm = el("button", {
        type: "button", className: "remove-btn",
        onclick: function () { delete state.lanes[laneId]; persistDraft(); renderLanes(); renderJobs(); },
      }, ["× Remove"]);
      list.appendChild(el("li", {}, [
        el("label", {}, ["id "]), idInput,
        el("label", {}, [" adapter "]), adapterInput,
        el("label", {}, [" display_model "]), modelInput,
        rm,
      ]));
    });
    container.appendChild(list);
    container.appendChild(el("button", {
      type: "button", className: "add-btn",
      onclick: function () {
        var id = "lane_" + (Object.keys(state.lanes).length + 1);
        state.lanes[id] = {
          adapter: "process",
          display_model: "",
          command: ["echo"],
          capabilities: ["write"],
        };
        persistDraft(); renderLanes(); renderJobs();
      },
    }, ["+ Add lane"]));
  }

  function renderJobs() {
    var container = document.getElementById("edit-jobs");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Jobs"]));
    state.jobs = state.jobs || [];
    state.jobs.forEach(function (job, index) {
      job = job || {};
      var card = el("details", { className: "job-card" });
      var summary = el("summary", {}, [
        el("strong", {}, [job.id || "(no id)"]),
        " — type=" + (job.type || "?"),
        ", role=" + (job.role_id || "?"),
        ", lane=" + (job.lane_id || "?"),
      ]);
      card.appendChild(summary);
      var body = el("div", { className: "job-card-body" });
      var dl = el("dl", { className: "kv-grid" });
      function addField(label, control, fieldPath) {
        dl.appendChild(el("dt", {}, [label]));
        var dd = el("dd", fieldPath ? { "data-field-path": fieldPath } : {}, [control]);
        dl.appendChild(dd);
      }
      addField("id", input(job.id, function (v) { state.jobs[index].id = v; persistDraft(); summary.firstChild.textContent = v || "(no id)"; }), "jobs[" + index + "].id");
      addField("type", select(job.type || "generic", JOB_TYPES, function (v) {
        state.jobs[index].type = v; persistDraft(); renderJobs();
      }));
      addField("title", input(job.title, function (v) { state.jobs[index].title = v; persistDraft(); }));
      addField("objective", textarea(job.objective, function (v) { state.jobs[index].objective = v; persistDraft(); }));
      var roleOpts = [""].concat(Object.keys(state.roles || {}));
      addField("role_id", select(job.role_id || "", roleOpts, function (v) { state.jobs[index].role_id = v; persistDraft(); }), "jobs[" + index + "].role_id");
      var laneOpts = [""].concat(Object.keys(state.lanes || {}));
      addField("lane_id", select(job.lane_id || "", laneOpts, function (v) { state.jobs[index].lane_id = v; persistDraft(); }), "jobs[" + index + "].lane_id");
      addField("task_prompt.path", input(((job.task_prompt || {}).path) || "", function (v) {
        state.jobs[index].task_prompt = state.jobs[index].task_prompt || {};
        state.jobs[index].task_prompt.path = v;
        persistDraft();
      }));
      addField("write_scope.mode", input(((job.write_scope || {}).mode) || "repo_write", function (v) {
        state.jobs[index].write_scope = state.jobs[index].write_scope || {};
        state.jobs[index].write_scope.mode = v;
        persistDraft();
      }));
      addField("write_scope.allowed_paths (comma-separated)",
        input((((job.write_scope || {}).allowed_paths) || []).join(","), function (v) {
          state.jobs[index].write_scope = state.jobs[index].write_scope || {};
          state.jobs[index].write_scope.allowed_paths = v.split(",").map(function (s) { return s.trim(); }).filter(Boolean);
          persistDraft();
        }));
      addField("write_scope.forbidden_paths (comma-separated)",
        input((((job.write_scope || {}).forbidden_paths) || []).join(","), function (v) {
          state.jobs[index].write_scope = state.jobs[index].write_scope || {};
          state.jobs[index].write_scope.forbidden_paths = v.split(",").map(function (s) { return s.trim(); }).filter(Boolean);
          persistDraft();
        }));
      if (job.type === "review") {
        var postureValue = job.review_posture || "";
        var postureOpts = [""].concat(ALLOWED_POSTURES);
        addField("review_posture", select(postureValue, postureOpts, function (v) {
          if (v) state.jobs[index].review_posture = v;
          else delete state.jobs[index].review_posture;
          persistDraft();
        }));
        addField("reviewer_access_scope", input(job.reviewer_access_scope, function (v) {
          if (v) state.jobs[index].reviewer_access_scope = v;
          else delete state.jobs[index].reviewer_access_scope;
          persistDraft();
        }));
        addField("reviewer_context_policy", input(job.reviewer_context_policy, function (v) {
          if (v) state.jobs[index].reviewer_context_policy = v;
          else delete state.jobs[index].reviewer_context_policy;
          persistDraft();
        }));
        addField("fresh_session_required", select(
          job.fresh_session_required === true ? "true" : (job.fresh_session_required === false ? "false" : ""),
          ["", "true", "false"], function (v) {
            if (v === "true") state.jobs[index].fresh_session_required = true;
            else if (v === "false") state.jobs[index].fresh_session_required = false;
            else delete state.jobs[index].fresh_session_required;
            persistDraft();
          }));
      }
      if (job.type === "build") {
        addField("required_review_postures (comma-separated)",
          input(((job.required_review_postures) || []).join(","), function (v) {
            var list = v.split(",").map(function (s) { return s.trim(); }).filter(Boolean);
            if (list.length) state.jobs[index].required_review_postures = list;
            else delete state.jobs[index].required_review_postures;
            persistDraft();
          }));
      }
      // Expected artifacts: simplified view.
      addField("expected_artifacts (count)", el("span", { className: "muted" }, [
        String((job.expected_artifacts || []).length),
        " — edit raw via the workflow.json file for now",
      ]));
      var rm = el("button", {
        type: "button", className: "remove-btn",
        onclick: function () { state.jobs.splice(index, 1); persistDraft(); renderJobs(); },
      }, ["× Remove job"]);
      body.appendChild(dl);
      body.appendChild(rm);
      card.appendChild(body);
      container.appendChild(card);
    });
    container.appendChild(el("button", {
      type: "button", className: "add-btn",
      onclick: function () {
        state.jobs = state.jobs || [];
        state.jobs.push({
          id: "job_" + (state.jobs.length + 1),
          type: "generic", title: "", role_id: "", lane_id: "",
          objective: "",
          task_prompt: { path: "" },
          write_scope: { mode: "repo_write", allowed_paths: [], forbidden_paths: [".striatum/"] },
          expected_artifacts: [],
        });
        persistDraft(); renderJobs();
      },
    }, ["+ Add job"]));
  }

  function renderEdges() {
    var container = document.getElementById("edit-edges");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Edges"]));
    state.edges = state.edges || [];
    var jobIds = (state.jobs || []).map(function (j) { return j && j.id ? j.id : ""; }).filter(Boolean);
    var list = el("ul", { className: "edit-list" });
    state.edges.forEach(function (edge, index) {
      edge = edge || {};
      var fromSel = select(edge.from || "", [""].concat(jobIds), function (v) { state.edges[index].from = v; persistDraft(); });
      var toSel = select(edge.to || "", [""].concat(jobIds), function (v) { state.edges[index].to = v; persistDraft(); });
      var onSel = select(edge.on || "completed", EDGE_ON, function (v) { state.edges[index].on = v; persistDraft(); });
      var rm = el("button", {
        type: "button", className: "remove-btn",
        onclick: function () { state.edges.splice(index, 1); persistDraft(); renderEdges(); },
      }, ["× Remove"]);
      list.appendChild(el("li", {}, [
        el("label", {}, ["from "]), fromSel,
        el("label", {}, [" to "]), toSel,
        el("label", {}, [" on "]), onSel,
        rm,
      ]));
    });
    container.appendChild(list);
    container.appendChild(el("button", {
      type: "button", className: "add-btn",
      onclick: function () {
        state.edges.push({ from: "", to: "", on: "completed" });
        persistDraft(); renderEdges();
      },
    }, ["+ Add edge"]));
  }

  function renderCycles() {
    var container = document.getElementById("edit-cycles");
    container.innerHTML = "";
    container.appendChild(el("h2", {}, ["Cycles"]));
    state.cycles = state.cycles || [];
    var jobIds = (state.jobs || []).map(function (j) { return j && j.id ? j.id : ""; }).filter(Boolean);
    var list = el("ul", { className: "edit-list" });
    state.cycles.forEach(function (cycle, index) {
      cycle = cycle || {};
      var fromSel = select(cycle.from || "", [""].concat(jobIds), function (v) { state.cycles[index].from = v; persistDraft(); });
      var toSel = select(cycle.to || "", [""].concat(jobIds), function (v) { state.cycles[index].to = v; persistDraft(); });
      var maxIter = input(cycle.max_iterations == null ? "1" : String(cycle.max_iterations), function (v) {
        var n = parseInt(v, 10);
        if (!isNaN(n) && n >= 1) state.cycles[index].max_iterations = n;
        persistDraft();
      });
      var rm = el("button", {
        type: "button", className: "remove-btn",
        onclick: function () { state.cycles.splice(index, 1); persistDraft(); renderCycles(); },
      }, ["× Remove"]);
      list.appendChild(el("li", {}, [
        el("label", {}, ["from "]), fromSel,
        el("label", {}, [" to "]), toSel,
        el("label", {}, [" max_iterations "]), maxIter,
        rm,
      ]));
    });
    container.appendChild(list);
    container.appendChild(el("button", {
      type: "button", className: "add-btn",
      onclick: function () {
        state.cycles.push({ from: "", to: "", on_verdict: "needs_revision", max_iterations: 1 });
        persistDraft(); renderCycles();
      },
    }, ["+ Add cycle"]));
  }

  function showError(message) {
    var banner = document.getElementById("edit-error-banner");
    if (!banner) return;
    banner.textContent = message;
    banner.hidden = false;
  }

  function clearError() {
    var banner = document.getElementById("edit-error-banner");
    if (!banner) return;
    banner.hidden = true;
    banner.textContent = "";
  }

  function setStatus(text) {
    var status = document.getElementById("edit-status");
    if (status) status.textContent = text || "";
  }

  function clearFieldErrors() {
    var marked = document.querySelectorAll(".field-error");
    for (var i = 0; i < marked.length; i++) {
      marked[i].classList.remove("field-error");
      marked[i].removeAttribute("title");
    }
  }

  function highlightFieldErrors(errors) {
    if (!errors || !errors.length) return;
    for (var i = 0; i < errors.length; i++) {
      var fp = errors[i].field_path;
      if (!fp) continue;
      var node = document.querySelector('[data-field-path="' + fp.replace(/"/g, '\\"') + '"]');
      if (node) {
        node.classList.add("field-error");
        node.setAttribute("title", errors[i].message || "");
      }
    }
  }

  function save() {
    clearError();
    clearFieldErrors();
    setStatus("Saving…");
    var saveBtn = document.getElementById("edit-save-btn");
    if (saveBtn) saveBtn.disabled = true;
    var headers = { "Content-Type": "application/json" };
    if (diskSha256) headers["If-Match"] = '"' + diskSha256 + '"';
    fetch("/workflows/edit/" + relPath, {
      method: "POST",
      headers: headers,
      body: JSON.stringify(state),
    })
      .then(function (resp) {
        if (resp.status === 200) {
          clearDraft();
          window.location.href = "/workflows/" + relPath;
        } else if (resp.status === 412) {
          return resp.json().then(function (body) {
            var msg = "File changed on disk. Reload to see the new contents (your draft is saved in this browser).";
            showError(msg);
            setStatus("");
          });
        } else {
          return resp.json().then(function (body) {
            var err = body && body.error;
            var msg = (err && err.message) || ("Save failed (" + resp.status + ")");
            showError(msg);
            if (err && err.errors) highlightFieldErrors(err.errors);
            setStatus("");
          }, function () {
            showError("Save failed (" + resp.status + ")");
            setStatus("");
          });
        }
      })
      .catch(function (err) {
        showError("Network error: " + err.message);
        setStatus("");
      })
      .finally(function () {
        if (saveBtn) saveBtn.disabled = false;
      });
  }

  function checkDraftRecovery() {
    try {
      var raw = localStorage.getItem("workflow_edit_" + relPath);
      if (!raw) return;
      var draft = JSON.parse(raw);
      if (JSON.stringify(draft) === JSON.stringify(state)) return;
      if (window.confirm("Recover unsaved changes for this workflow?")) {
        state = draft;
      } else {
        clearDraft();
      }
    } catch (err) { /* ignore */ }
  }

  checkDraftRecovery();

  document.getElementById("edit-save-btn").addEventListener("click", save);
  document.getElementById("edit-cancel-btn").addEventListener("click", function () {
    if (window.confirm("Discard changes? (localStorage draft will be cleared)")) {
      clearDraft();
      window.location.href = "/workflows/" + relPath;
    }
  });

  renderHeader();
  renderRoles();
  renderLanes();
  renderJobs();
  renderEdges();
  renderCycles();
})();
