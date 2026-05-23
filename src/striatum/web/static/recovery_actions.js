(function () {
  function postJson(url, body) {
    return fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    }).then(function (resp) {
      return resp.json().then(function (payload) {
        return { status: resp.status, payload: payload };
      });
    });
  }

  var buttons = document.querySelectorAll("[data-recovery-action]");
  for (var i = 0; i < buttons.length; i++) {
    var button = buttons[i];
    button.addEventListener("click", (function (btn) {
      return function () {
        var action = btn.getAttribute("data-recovery-action");
        var host = btn.closest("[data-run-id]");
        var runId = host && host.getAttribute("data-run-id");
        if (!action || !runId) return;
        var body = {};
        if (action === "auto-finalize") {
          body.dry_run = btn.getAttribute("data-dry-run") !== "false";
        }
        if (action === "resume") {
          body.blocker_id = btn.getAttribute("data-blocker-id");
          body.force = true;
        }
        if (action === "sweep" && !window.confirm("Run recovery sweep for this run?")) return;
        btn.disabled = true;
        postJson("/run/" + encodeURIComponent(runId) + "/recovery/" + encodeURIComponent(action), body)
          .then(function (result) {
            if (result.status === 200 && result.payload && result.payload.ok) {
              window.location.reload();
              return;
            }
            var message = result.payload && result.payload.error && result.payload.error.message;
            window.alert("Recovery action failed: " + (message || result.status));
            btn.disabled = false;
          })
          .catch(function (err) {
            window.alert("Network error: " + err.message);
            btn.disabled = false;
          });
      };
    })(button));
  }
})();
