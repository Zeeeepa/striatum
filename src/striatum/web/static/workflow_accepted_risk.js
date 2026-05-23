// TODO 55: daemon-backed accepted workflow-lint risk form.

(function () {
  "use strict";

  function text(value) {
    return value == null ? "" : String(value);
  }

  function encodeWorkflowPath(path) {
    return text(path).split("/").map(encodeURIComponent).join("/");
  }

  function setError(node, message) {
    if (!node) return;
    if (!message) {
      node.hidden = true;
      node.textContent = "";
      return;
    }
    node.hidden = false;
    node.textContent = message;
  }

  document.addEventListener("DOMContentLoaded", function () {
    var dialog = document.getElementById("accept-risk-dialog");
    var form = document.getElementById("accept-risk-form");
    if (!dialog || !form) return;

    var findingSummary = document.getElementById("accept-risk-finding");
    var errorNode = document.getElementById("accept-risk-error");
    var submit = form.querySelector("button[type='submit']");
    var cancel = form.querySelector("[data-accept-risk-cancel]");
    var fingerprintInput = form.querySelector("[name='finding_fingerprint_sha256']");
    var workflowPathInput = form.querySelector("[name='workflow_path']");

    function closeDialog() {
      setError(errorNode, "");
      if (typeof dialog.close === "function") {
        dialog.close();
      } else {
        dialog.removeAttribute("open");
      }
    }

    if (cancel) {
      cancel.addEventListener("click", closeDialog);
    }
    dialog.addEventListener("cancel", function () {
      setError(errorNode, "");
    });

    document.querySelectorAll("[data-accept-risk-open]").forEach(function (button) {
      button.addEventListener("click", function () {
        var fingerprint = button.getAttribute("data-fingerprint") || "";
        var workflowPath = button.getAttribute("data-workflow-path") || "";
        var rule = button.getAttribute("data-rule") || "workflow lint";
        var message = button.getAttribute("data-message") || "";
        fingerprintInput.value = fingerprint;
        workflowPathInput.value = workflowPath;
        findingSummary.textContent = rule + " · " + fingerprint.slice(0, 12) + "\n" + message;
        setError(errorNode, "");
        if (typeof dialog.showModal === "function") {
          dialog.showModal();
        } else {
          dialog.setAttribute("open", "");
        }
      });
    });

    form.addEventListener("submit", function (event) {
      event.preventDefault();
      var formData = new FormData(form);
      var workflowPath = text(formData.get("workflow_path"));
      var payload = {
        finding_fingerprint_sha256: text(formData.get("finding_fingerprint_sha256")).trim(),
        decision_artifact_ref: text(formData.get("decision_artifact_ref")).trim(),
        rationale: text(formData.get("rationale")).trim(),
        accepted_by: text(formData.get("accepted_by")).trim(),
      };
      if (!payload.finding_fingerprint_sha256 || !payload.decision_artifact_ref || !payload.rationale) {
        setError(errorNode, "Finding fingerprint, decision artifact reference, and rationale are required.");
        return;
      }
      if (submit) submit.disabled = true;
      setError(errorNode, "");
      fetch("/workflows/accept-risk/" + encodeWorkflowPath(workflowPath), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }).then(function (response) {
        return response.json().then(function (body) {
          if (response.status === 200 && body.ok) {
            window.location.reload();
            return;
          }
          var message = body && body.error && body.error.message;
          setError(errorNode, "Accept risk failed: " + (message || response.status));
          if (submit) submit.disabled = false;
        });
      }).catch(function (err) {
        setError(errorNode, "Network error: " + err.message);
        if (submit) submit.disabled = false;
      });
    });
  });
}());
