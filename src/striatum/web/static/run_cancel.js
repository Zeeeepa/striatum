// RFC 0024 V3: cancel-run button on /run/<id>.
// POSTs {} to /run/<id>/cancel; on 200 reloads.

(function () {
  var btn = document.getElementById("run-cancel-btn");
  if (!btn) return;
  btn.addEventListener("click", function () {
    if (!window.confirm("Cancel this run? In-flight work will be marked canceled.")) return;
    btn.disabled = true;
    var runId = btn.getAttribute("data-run-id");
    fetch("/run/" + runId + "/cancel", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }).then(function (resp) {
      return resp.json().then(function (body) {
        if (resp.status === 200 && body.ok) {
          window.location.reload();
        } else {
          var msg = body && body.error && body.error.message;
          window.alert("Cancel failed: " + (msg || resp.status));
          btn.disabled = false;
        }
      });
    }).catch(function (err) {
      window.alert("Network error: " + err.message);
      btn.disabled = false;
    });
  });
})();
