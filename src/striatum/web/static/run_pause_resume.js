// RFC 0024 V4: pause/resume buttons on /run/<id>.
// POSTs {} to /run/<id>/(pause|resume); on 200 reloads.

(function () {
  var btns = document.querySelectorAll("[data-action='pause'], [data-action='resume']");
  for (var i = 0; i < btns.length; i++) {
    var btn = btns[i];
    btn.addEventListener("click", (function (b) {
      return function () {
        var action = b.getAttribute("data-action");
        var runId = b.getAttribute("data-run-id");
        b.disabled = true;
        fetch("/run/" + runId + "/" + action, {
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
