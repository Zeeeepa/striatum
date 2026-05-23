(function () {
  "use strict";

  const storage = {
    read(key, fallback, allowed) {
      try {
        const raw = window.localStorage.getItem(key);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw);
        if (!parsed || parsed.version !== 1) return fallback;
        if (allowed && !allowed.includes(parsed.value)) return fallback;
        return parsed.value;
      } catch (_err) {
        return fallback;
      }
    },
    write(key, value) {
      try {
        window.localStorage.setItem(key, JSON.stringify({ version: 1, value }));
      } catch (_err) {
        // localStorage can be unavailable in private modes; rendering must continue.
      }
    },
    readObject(key, fallback) {
      try {
        const raw = window.localStorage.getItem(key);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw);
        if (!parsed || parsed.version !== 1) return fallback;
        return parsed;
      } catch (_err) {
        return fallback;
      }
    },
    writeObject(key, value) {
      try {
        window.localStorage.setItem(key, JSON.stringify({ version: 1, ...value }));
      } catch (_err) {
        // See write().
      }
    },
  };

  function parseDate(value) {
    if (!value) return null;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return null;
    return date;
  }

  function formatDuration(ms) {
    if (!Number.isFinite(ms) || ms < 0) return "-";
    const totalSeconds = Math.floor(ms / 1000);
    if (totalSeconds < 60) return `${totalSeconds}s`;
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    if (minutes < 60) return `${minutes}m ${seconds}s`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m`;
  }

  function formatDurationBetween(startValue, endValue) {
    const start = parseDate(startValue);
    const end = endValue ? parseDate(endValue) : new Date();
    if (!start || !end) return "-";
    return formatDuration(end.getTime() - start.getTime());
  }

  function isEditableTarget(target) {
    if (!(target instanceof Element)) return false;
    return Boolean(target.closest("input, textarea, select, button, [contenteditable], dialog[open]"));
  }

  function renderTimes(mode) {
    const formatter = new Intl.DateTimeFormat(undefined, {
      year: "numeric",
      month: "short",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
    document.querySelectorAll("time[datetime]").forEach((node) => {
      const raw = node.getAttribute("datetime") || "";
      if (mode === "utc") {
        node.textContent = raw || node.textContent;
        return;
      }
      const date = parseDate(raw);
      if (!date) return;
      try {
        node.textContent = formatter.format(date);
      } catch (_err) {
        // Keep the server-rendered timestamp.
      }
    });
  }

  function initTimeToggle() {
    const button = document.querySelector("#time-mode-toggle");
    if (!button) return;
    let mode = storage.read("striatum.ui.time.mode", "utc", ["utc", "local"]);
    const apply = () => {
      button.textContent = mode === "local" ? "Local" : "UTC";
      button.setAttribute("aria-pressed", mode === "local" ? "true" : "false");
      renderTimes(mode);
    };
    button.addEventListener("click", () => {
      mode = mode === "local" ? "utc" : "local";
      storage.write("striatum.ui.time.mode", mode);
      apply();
    });
    apply();
  }

  function openShortcutHelp(dialog) {
    if (dialog && typeof dialog.showModal === "function" && !dialog.open) {
      dialog.showModal();
    }
  }

  function initShortcuts() {
    const dialog = document.querySelector("#shortcut-help");
    const helpButton = document.querySelector("#shortcut-help-button");
    if (helpButton) helpButton.addEventListener("click", () => openShortcutHelp(dialog));
    if (dialog) {
      dialog.addEventListener("click", (event) => {
        const rect = dialog.getBoundingClientRect();
        const outside = event.clientX < rect.left || event.clientX > rect.right
          || event.clientY < rect.top || event.clientY > rect.bottom;
        if (outside) dialog.close();
      });
    }

    let gSeenAt = 0;
    const routes = { r: "/", w: "/workflows", e: "/escalations", x: "/cross-repo", c: "/chat", d: "/doctor" };
    document.addEventListener("keydown", (event) => {
      if (isEditableTarget(event.target)) return;
      if (event.key === "?") {
        event.preventDefault();
        openShortcutHelp(dialog);
        return;
      }
      const now = Date.now();
      if (event.key.toLowerCase() === "g") {
        gSeenAt = now;
        return;
      }
      const key = event.key.toLowerCase();
      if (gSeenAt && now - gSeenAt <= 1000 && routes[key]) {
        event.preventDefault();
        window.location.assign(routes[key]);
      }
      gSeenAt = 0;
    });
  }

  let copyOnClickAsset = null;

  function loadCopyOnClick() {
    if (window.StriatumCopyOnClick) return Promise.resolve(window.StriatumCopyOnClick);
    if (copyOnClickAsset) return copyOnClickAsset;
    copyOnClickAsset = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.defer = true;
      script.src = "/static/copy_on_click.js";
      script.onload = () => resolve(window.StriatumCopyOnClick);
      script.onerror = reject;
      document.head.appendChild(script);
    });
    return copyOnClickAsset;
  }

  function initCopyOnClick() {
    loadCopyOnClick().then((api) => {
      if (api && typeof api.init === "function") api.init(document);
    }).catch((_err) => {
      // Copy controls are progressive enhancement; the page should keep working.
    });
  }

  window.StriatumUI = {
    storage,
    parseDate,
    formatDuration,
    formatDurationBetween,
    isEditableTarget,
    renderTimes,
    initCopyOnClick,
  };

  document.addEventListener("DOMContentLoaded", () => {
    initTimeToggle();
    initShortcuts();
    initCopyOnClick();
  });
}());
