// RFC 0023 V1: chat-page island. Subscribes to the SSE event stream
// and appends new transcript entries; intercepts the form submit so
// the textarea clears immediately and the user sees the message
// before the network round-trip completes.
(function () {
  var page = document.querySelector(".chat-page");
  if (!page) return;
  var sessionId = page.getAttribute("data-session-id");
  if (!sessionId) return;

  var history = document.getElementById("chat-history");
  var form = document.querySelector(".chat-input-form");
  var statusSpan = document.getElementById("chat-status");

  function appendMessage(role, body, createdAt) {
    if (!history) return;
    var article = document.createElement("article");
    article.className = "chat-message chat-role-" + role;
    var header = document.createElement("header");
    header.className = "chat-message-meta";
    var roleSpan = document.createElement("strong");
    roleSpan.textContent = role;
    header.appendChild(roleSpan);
    if (createdAt) {
      var ts = document.createElement("span");
      ts.className = "muted";
      ts.textContent = createdAt;
      header.appendChild(ts);
    }
    article.appendChild(header);
    var bodyDiv = document.createElement("div");
    bodyDiv.className = "chat-message-body";
    // body may be HTML (from server rendering) — we only use this for
    // user messages we just added, so plain text is fine.
    bodyDiv.textContent = body;
    article.appendChild(bodyDiv);
    history.appendChild(article);
    history.scrollTop = history.scrollHeight;
  }

  function setStatus(text) {
    if (statusSpan) statusSpan.textContent = text || "";
  }

  if (form) {
    var textarea = form.querySelector("textarea[name='message']");
    var submitBtn = form.querySelector("button[type='submit']");
    form.addEventListener("submit", function (e) {
      e.preventDefault();
      if (!textarea || !textarea.value.trim()) return;
      var message = textarea.value;
      var formData = new FormData();
      formData.append("message", message);
      submitBtn.disabled = true;
      setStatus("Sending…");
      appendMessage("user", message, "");
      textarea.value = "";
      fetch("/chat/" + encodeURIComponent(sessionId) + "/send", {
        method: "POST",
        body: formData,
      })
        .then(function (resp) {
          if (!resp.ok) {
            setStatus("Error: " + resp.status);
          } else {
            setStatus("Streaming response…");
          }
        })
        .catch(function (err) {
          setStatus("Network error: " + err.message);
        })
        .finally(function () {
          submitBtn.disabled = false;
        });
    });

    // Shift+Enter for newline, Enter to submit.
    if (textarea) {
      textarea.addEventListener("keydown", function (e) {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          form.requestSubmit();
        }
      });
    }
  }

  // SSE feed for incoming transcript events.
  if (typeof EventSource !== "undefined") {
    var es = new EventSource("/chat/" + encodeURIComponent(sessionId) + "/events");
    var streamingDiv = null;
    es.addEventListener("message", function (e) {
      try {
        var data = JSON.parse(e.data);
        if (data.role && data.content) {
          if (data.role === "assistant" && data.streaming) {
            // Append to the current streaming message.
            if (!streamingDiv) {
              streamingDiv = document.createElement("article");
              streamingDiv.className = "chat-message chat-role-assistant chat-streaming";
              var header = document.createElement("header");
              header.className = "chat-message-meta";
              var rs = document.createElement("strong");
              rs.textContent = "assistant";
              header.appendChild(rs);
              streamingDiv.appendChild(header);
              var bodyDiv = document.createElement("div");
              bodyDiv.className = "chat-message-body";
              streamingDiv.appendChild(bodyDiv);
              history.appendChild(streamingDiv);
            }
            var bd = streamingDiv.querySelector(".chat-message-body");
            bd.textContent = (bd.textContent || "") + data.content;
            history.scrollTop = history.scrollHeight;
          } else {
            appendMessage(data.role, data.content, data.created_at || "");
            streamingDiv = null;
            setStatus("");
          }
        }
      } catch (err) { /* ignore non-JSON */ }
    });
    es.addEventListener("error", function () {
      setStatus("Disconnected; reload the page to retry.");
    });
  }
})();
