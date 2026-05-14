// RFC 0050 V2: operator override-verdict modal for job detail pages.

(function () {
  var ALLOWED_VERDICTS = ["accept", "accept_with_findings"];
  var FOCUSABLE_SELECTOR = [
    "a[href]",
    "button:not([disabled])",
    "input:not([disabled])",
    "select:not([disabled])",
    "textarea:not([disabled])",
    "[tabindex]:not([tabindex='-1'])",
  ].join(",");

  function allowedVerdict(value) {
    return ALLOWED_VERDICTS.indexOf(value) !== -1;
  }

  function collectPayload(form) {
    var verdict = String(form.elements.verdict.value || "");
    var rationale = String(form.elements.rationale.value || "").trim();
    var findingsArtifactId = String(form.elements.findings_artifact_id.value || "").trim();
    var autoFresh = !!form.elements.auto_fresh_session.checked;
    var payload = {
      verdict: allowedVerdict(verdict) ? verdict : "accept",
      rationale: rationale,
      auto_fresh_session: autoFresh,
    };
    if (findingsArtifactId) {
      payload.findings_artifact_id = findingsArtifactId;
    }
    return payload;
  }

  function buildArgv(context, payload) {
    var argv = [
      "override-verdict",
      "--session-id",
      context.sessionId,
      "--job-id",
      context.jobId,
    ];
    if (payload.auto_fresh_session) {
      argv.push("--auto-fresh-session");
    }
    argv.push("--verdict", payload.verdict, "--rationale", payload.rationale);
    if (payload.findings_artifact_id) {
      argv.push("--findings-artifact-id", payload.findings_artifact_id);
    }
    argv.push("--json");
    return argv;
  }

  function postInvoke(argv) {
    return fetch("/v1/invoke", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ argv: argv }),
    }).then(function (response) {
      return response.json().catch(function () {
        return { ok: false, error: { message: response.statusText } };
      }).then(function (body) {
        body.http_status = response.status;
        return body;
      });
    });
  }

  function setStatus(node, message, isError) {
    node.textContent = message || "";
    node.classList.toggle("error", !!isError);
    node.hidden = !message;
  }

  function focusableElements(dialog) {
    return Array.prototype.slice.call(dialog.querySelectorAll(FOCUSABLE_SELECTOR))
      .filter(function (el) {
        return !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
      });
  }

  function trapFocus(dialog, event) {
    if (event.key !== "Tab") return;
    var nodes = focusableElements(dialog);
    if (nodes.length === 0) {
      event.preventDefault();
      return;
    }
    var first = nodes[0];
    var last = nodes[nodes.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function createDialog(host) {
    var dialog = document.createElement("dialog");
    var titleId = "override-verdict-title-" + Math.random().toString(16).slice(2);
    var descId = "override-verdict-desc-" + Math.random().toString(16).slice(2);
    dialog.className = "override-verdict-dialog";
    dialog.setAttribute("aria-modal", "true");
    dialog.setAttribute("aria-labelledby", titleId);
    dialog.setAttribute("aria-describedby", descId);
    dialog.innerHTML = [
      '<form method="dialog" class="override-verdict-form" novalidate>',
      '<h3 id="' + titleId + '">Override verdict</h3>',
      '<p id="' + descId + '" class="muted">Record an operator override verdict with a required rationale.</p>',
      '<label>Verdict',
      '<select name="verdict" required>',
      '<option value="accept">accept</option>',
      '<option value="accept_with_findings">accept_with_findings</option>',
      '</select>',
      '</label>',
      '<label>Findings artifact id',
      '<input name="findings_artifact_id" autocomplete="off">',
      '</label>',
      '<label class="checkbox-row">',
      '<input type="checkbox" name="auto_fresh_session" checked>',
      '<span>Auto fresh session</span>',
      '</label>',
      '<label>Rationale',
      '<textarea name="rationale" rows="5" required></textarea>',
      '</label>',
      '<p class="override-verdict-status" role="alert" hidden></p>',
      '<div class="modal-actions">',
      '<button type="button" class="secondary-button" data-override-close>Cancel</button>',
      '<button type="submit" class="primary-button" data-override-submit>Record override</button>',
      '</div>',
      '</form>',
    ].join("");
    host.appendChild(dialog);
    return dialog;
  }

  function initHost(host) {
    var trigger = host.querySelector("[data-override-verdict]");
    if (!trigger) return;
    var context = {
      jobId: host.getAttribute("data-job-id") || trigger.getAttribute("data-job-id") || "",
      sessionId: host.getAttribute("data-session-id") || trigger.getAttribute("data-session-id") || "",
    };
    var dialog = host.querySelector("dialog[data-override-dialog]") || createDialog(host);
    dialog.setAttribute("data-override-dialog", "true");
    var form = dialog.querySelector("form");
    var status = dialog.querySelector(".override-verdict-status");
    var closeButton = dialog.querySelector("[data-override-close]");
    var submitButton = dialog.querySelector("[data-override-submit]");
    var priorFocus = null;

    function closeDialog() {
      if (dialog.open) dialog.close();
      setStatus(status, "", false);
      if (priorFocus && typeof priorFocus.focus === "function") {
        priorFocus.focus();
      }
    }

    function openDialog() {
      if (!context.jobId || !context.sessionId) {
        window.alert("Override verdict requires server-rendered job and session identifiers.");
        return;
      }
      priorFocus = document.activeElement;
      if (typeof dialog.showModal === "function") {
        dialog.showModal();
      } else {
        dialog.setAttribute("open", "");
      }
      var initial = form.elements.verdict || form.elements.rationale || submitButton;
      if (initial && typeof initial.focus === "function") initial.focus();
    }

    trigger.disabled = !(context.jobId && context.sessionId);
    trigger.addEventListener("click", openDialog);
    closeButton.addEventListener("click", closeDialog);
    dialog.addEventListener("cancel", function (event) {
      event.preventDefault();
      closeDialog();
    });
    dialog.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        closeDialog();
        return;
      }
      trapFocus(dialog, event);
    });
    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var payload = collectPayload(form);
      if (!payload.rationale) {
        setStatus(status, "Rationale is required.", true);
        form.elements.rationale.focus();
        return;
      }
      submitButton.disabled = true;
      setStatus(status, "Recording override...", false);
      postInvoke(buildArgv(context, payload)).then(function (result) {
        if (result.ok) {
          setStatus(status, "Override recorded.", false);
          window.location.reload();
          return;
        }
        var error = result.error && result.error.message ? result.error.message : "override failed";
        setStatus(status, error, true);
        submitButton.disabled = false;
      }).catch(function (error) {
        setStatus(status, error.message || "network error", true);
        submitButton.disabled = false;
      });
    });
  }

  window.StriatumOverrideVerdict = {
    collectPayload: collectPayload,
    buildArgv: buildArgv,
    initHost: initHost,
  };

  var hosts = document.querySelectorAll('[data-modal="override-verdict"]');
  for (var i = 0; i < hosts.length; i++) {
    initHost(hosts[i]);
  }
})();
