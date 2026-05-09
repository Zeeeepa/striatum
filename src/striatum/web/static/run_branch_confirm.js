// RFC 0024 V2.1: branch-confirm button on /run/<id>.
// POSTs {branch, create, use_current} to /run/<id>/branch-confirm;
// on 200 reloads to show the now-running run.

(function () {
  var panel = document.querySelector(".branch-confirm-panel");
  if (!panel) return;
  var runId = panel.getAttribute("data-run-id");
  var btn = document.getElementById("bc-confirm-btn");
  var statusEl = document.getElementById("bc-status");
  if (!btn) return;
  btn.addEventListener("click", function () {
    btn.disabled = true;
    statusEl.textContent = "Confirming…";
    var branch = (document.getElementById("bc-branch") || {}).value || "";
    var create = (document.getElementById("bc-create") || {}).checked;
    var useCurrent = (document.getElementById("bc-use-current") || {}).checked;
    fetch("/run/" + runId + "/branch-confirm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ branch: branch, create: create, use_current: useCurrent }),
    }).then(function (resp) {
      return resp.json().then(function (body) {
        if (resp.status === 200 && body.ok) {
          window.location.reload();
        } else {
          var msg = body && body.error && body.error.message;
          statusEl.textContent = "";
          window.alert("Confirm failed: " + (msg || resp.status));
          btn.disabled = false;
        }
      });
    }).catch(function (err) {
      statusEl.textContent = "";
      window.alert("Network error: " + err.message);
      btn.disabled = false;
    });
  });
})();
