// RFC 0050 CLI-retirement slice: escalation resolve on the local web UI.

(function () {
  var form = document.querySelector("[data-escalation-resolve-form]");
  if (!form) return;
  form.addEventListener("submit", function (event) {
    event.preventDefault();
    var escalationId = form.getAttribute("data-escalation-id");
    var button = form.querySelector("button[type='submit']");
    var status = form.querySelector("[data-escalation-resolve-status]");
    var decisionInput = form.querySelector("[name='decision_id']");
    var noteInput = form.querySelector("[name='resolution_note']");
    var payload = {};
    var decisionId = decisionInput && decisionInput.value ? decisionInput.value.trim() : "";
    var resolutionNote = noteInput && noteInput.value ? noteInput.value.trim() : "";
    if (decisionId) payload.decision_id = decisionId;
    if (resolutionNote) payload.resolution_note = resolutionNote;
    if (button) button.disabled = true;
    if (status) {
      status.hidden = false;
      status.textContent = "Resolving...";
    }
    fetch("/escalations/" + encodeURIComponent(escalationId) + "/resolve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    }).then(function (resp) {
      return resp.json().then(function (body) {
        if (resp.status === 200 && body.ok) {
          window.location.reload();
          return;
        }
        var msg = body && body.error && body.error.message;
        if (status) status.textContent = "Resolve failed: " + (msg || resp.status);
        if (button) button.disabled = false;
      });
    }).catch(function (err) {
      if (status) status.textContent = "Network error: " + err.message;
      if (button) button.disabled = false;
    });
  });
})();
