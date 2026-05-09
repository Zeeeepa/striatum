// RFC 0024 V2: "Run this workflow now" button.
// POSTs {} to /workflows/run/<path>; on 200 navigates to /run/<run_id>.

(function () {
  var btn = document.getElementById("run-now-btn");
  if (!btn) return;
  btn.addEventListener("click", function () {
    btn.disabled = true;
    var rel = btn.getAttribute("data-rel-path");
    fetch("/workflows/run/" + rel, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    }).then(function (resp) {
      return resp.json().then(function (body) {
        if (resp.status === 200 && body.ok) {
          window.location.href = "/run/" + body.data.run_id;
        } else {
          var msg = body && body.error && body.error.message;
          window.alert("Run failed: " + (msg || resp.status));
          btn.disabled = false;
        }
      });
    }).catch(function (err) {
      window.alert("Network error: " + err.message);
      btn.disabled = false;
    });
  });
})();
