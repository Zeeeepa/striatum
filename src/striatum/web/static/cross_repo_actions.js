(function () {
  var buttons = document.querySelectorAll("[data-cross-repo-cancel]");
  for (var i = 0; i < buttons.length; i++) {
    var button = buttons[i];
    button.addEventListener("click", (function (btn) {
      return function () {
        var runId = btn.getAttribute("data-cross-repo-run-id");
        if (!runId) return;
        var reason = window.prompt("Cancel reason", "operator canceled via web");
        if (reason === null) return;
        btn.disabled = true;
        fetch("/cross-repo/" + encodeURIComponent(runId) + "/cancel", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason: reason }),
        }).then(function (resp) {
          return resp.json().then(function (payload) {
            if (resp.status === 200 && payload.ok) {
              window.location.reload();
              return;
            }
            var message = payload && payload.error && payload.error.message;
            window.alert("Cross-repo cancel failed: " + (message || resp.status));
            btn.disabled = false;
          });
        }).catch(function (err) {
          window.alert("Network error: " + err.message);
          btn.disabled = false;
        });
      };
    })(button));
  }
})();
