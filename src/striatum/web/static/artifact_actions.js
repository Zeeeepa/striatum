(function () {
  var panel = document.querySelector(".commit-apply-panel");
  if (!panel) return;
  var button = document.getElementById("commit-apply-btn");
  var input = document.getElementById("commit-apply-request-id");
  var status = document.getElementById("commit-apply-status");
  if (!button || !input) return;
  button.addEventListener("click", function () {
    var runId = panel.getAttribute("data-run-id");
    var artifactId = panel.getAttribute("data-artifact-id");
    var requestId = input.value.trim();
    if (!runId || !artifactId || !requestId) {
      window.alert("Request ID is required.");
      return;
    }
    if (!window.confirm("Create a local git commit from this confirmed request?")) return;
    button.disabled = true;
    if (status) status.textContent = "Applying...";
    fetch("/run/" + encodeURIComponent(runId) + "/artifact/" + encodeURIComponent(artifactId) + "/commit-apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirm_request_id: requestId }),
    }).then(function (resp) {
      return resp.json().then(function (payload) {
        if (resp.status === 200 && payload.ok) {
          if (status) status.textContent = "Local commit created.";
          return;
        }
        var message = payload && payload.error && payload.error.message;
        window.alert("Commit apply failed: " + (message || resp.status));
        if (status) status.textContent = "";
        button.disabled = false;
      });
    }).catch(function (err) {
      window.alert("Network error: " + err.message);
      if (status) status.textContent = "";
      button.disabled = false;
    });
  });
})();
