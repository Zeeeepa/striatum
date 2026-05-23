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

  var buttons = document.querySelectorAll("[data-supervise-action]");
  for (var i = 0; i < buttons.length; i++) {
    var button = buttons[i];
    button.addEventListener("click", (function (btn) {
      return function () {
        var action = btn.getAttribute("data-supervise-action");
        var sessionId = btn.getAttribute("data-session-id");
        if (!action || !sessionId) return;
        if (action === "stop" && !window.confirm("Stop this supervised process?")) return;
        btn.disabled = true;
        postJson("/sessions/" + encodeURIComponent(sessionId) + "/supervise/" + encodeURIComponent(action), {
          reason: "operator_stop_via_web",
        }).then(function (result) {
          if (result.status === 200 && result.payload && result.payload.ok) {
            window.location.reload();
            return;
          }
          var message = result.payload && result.payload.error && result.payload.error.message;
          window.alert("Supervisor action failed: " + (message || result.status));
          btn.disabled = false;
        }).catch(function (err) {
          window.alert("Network error: " + err.message);
          btn.disabled = false;
        });
      };
    })(button));
  }
})();
