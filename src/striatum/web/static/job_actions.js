// RFC 0024 V4: per-job cancel + retry buttons on /run/<id>/job/<jid>.

(function () {
  var btns = document.querySelectorAll("[data-job-action]");
  for (var i = 0; i < btns.length; i++) {
    var btn = btns[i];
    btn.addEventListener("click", (function (b) {
      return function () {
        var action = b.getAttribute("data-job-action");
        var runId = b.getAttribute("data-run-id");
        var jobId = b.getAttribute("data-job-id");
        if (action === "cancel") {
          if (!window.confirm("Cancel this job AND its dependents (jobs that depend on it)?")) return;
        } else if (action === "retry") {
          if (!window.confirm("Retry this job? It will be reset to queued and re-enqueued. If the run is canceled or failed, the run will be revived.")) return;
        }
        b.disabled = true;
        fetch("/run/" + runId + "/job/" + jobId + "/" + action, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
        }).then(function (resp) {
          return resp.json().then(function (body) {
            if (resp.status === 200 && body.ok) {
              window.location.reload();
            } else {
              var msg = body && body.error && body.error.message;
              window.alert(action + " failed: " + (msg || resp.status));
              b.disabled = false;
            }
          });
        }).catch(function (err) {
          window.alert("Network error: " + err.message);
          b.disabled = false;
        });
      };
    })(btn));
  }
})();
