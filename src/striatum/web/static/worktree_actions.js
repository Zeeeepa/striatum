(function () {
  var panels = document.querySelectorAll(".worktree-actions");
  for (var i = 0; i < panels.length; i++) {
    var panel = panels[i];
    var button = panel.querySelector("[data-worktree-action='create']");
    if (!button) continue;
    button.addEventListener("click", (function (root, btn) {
      return function () {
        var runId = root.getAttribute("data-run-id");
        var jobId = root.getAttribute("data-job-id");
        var sessionId = root.getAttribute("data-session-id");
        var leaseId = root.getAttribute("data-lease-id");
        if (!runId || !jobId || !sessionId || !leaseId) {
          window.alert("Worktree action is missing session or lease metadata.");
          return;
        }
        btn.disabled = true;
        fetch("/run/" + encodeURIComponent(runId) + "/job/" + encodeURIComponent(jobId) + "/worktree/create", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ session_id: sessionId, lease_id: leaseId }),
        }).then(function (resp) {
          return resp.json().then(function (payload) {
            if (resp.status === 200 && payload.ok) {
              window.location.reload();
              return;
            }
            var message = payload && payload.error && payload.error.message;
            window.alert("Worktree create failed: " + (message || resp.status));
            btn.disabled = false;
          });
        }).catch(function (err) {
          window.alert("Network error: " + err.message);
          btn.disabled = false;
        });
      };
    })(panel, button));
  }
})();
