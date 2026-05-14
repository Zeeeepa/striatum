(function () {
  "use strict";

  // GH #12: copy-on-click is restricted to an explicit closed allowlist of
  // container selectors. A raw `[data-copy]` injected outside these contexts
  // (e.g. by a malicious PR or XSS) gets no affordances and no activation, so
  // operator clicks cannot be coerced into copying attacker-controlled text.
  // The allowlist covers the two surfaces that ship `data-copy` today
  // (recovery recipe list, recovery auto-publish callout) plus the
  // future-facing copy-token / code-recipe classes called out in
  // docs/issues/12/SPEC.md so new copy surfaces opt in by class, not by
  // attribute alone.
  const ALLOWED_COPY_CONTAINER_SELECTORS = [
    ".recipe-list",
    ".recovery-auto-publish",
    ".code-recipe",
    ".copyable-token",
  ];
  const ALLOWED_COPY_CONTAINER_SELECTOR = ALLOWED_COPY_CONTAINER_SELECTORS.join(", ");

  const initializedRoots = new WeakSet();
  let toastTimer = null;

  function isAllowedCopyTarget(node) {
    if (!(node instanceof Element)) return false;
    return node.closest(ALLOWED_COPY_CONTAINER_SELECTOR) !== null;
  }

  function copyWithFallback(text) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "");
    textarea.className = "copy-on-click-buffer";
    Object.assign(textarea.style, {
      position: "fixed",
      left: "-9999px",
      top: "0",
    });
    document.body.appendChild(textarea);
    textarea.select();
    try {
      document.execCommand("copy");
      return Promise.resolve();
    } catch (err) {
      return Promise.reject(err);
    } finally {
      textarea.remove();
    }
  }

  function copyText(text) {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
      return navigator.clipboard.writeText(text);
    }
    return copyWithFallback(text);
  }

  function findCopyTarget(target) {
    if (!(target instanceof Element)) return null;
    const copyEl = target.closest("[data-copy]");
    if (!copyEl) return null;
    if (!isAllowedCopyTarget(copyEl)) return null;
    return copyEl;
  }

  function ensureToast() {
    let toast = document.querySelector("[data-copy-toast]");
    if (toast) return toast;
    toast = document.createElement("div");
    toast.className = "copy-toast";
    toast.setAttribute("data-copy-toast", "");
    toast.setAttribute("role", "status");
    toast.setAttribute("aria-live", "polite");
    Object.assign(toast.style, {
      position: "fixed",
      left: "50%",
      bottom: "1.5rem",
      zIndex: "1100",
      transform: "translateX(-50%)",
      padding: "0.5rem 0.75rem",
      border: "1px solid var(--border)",
      borderRadius: "var(--radius)",
      background: "var(--bg-elevated)",
      color: "var(--fg-primary)",
      boxShadow: "var(--shadow-md)",
    });
    toast.hidden = true;
    document.body.appendChild(toast);
    return toast;
  }

  function showToast(message) {
    const toast = ensureToast();
    toast.textContent = message;
    toast.hidden = false;
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => {
      toast.hidden = true;
    }, 1400);
  }

  function enhanceTargets(root) {
    root.querySelectorAll("[data-copy]").forEach((node) => {
      if (!isAllowedCopyTarget(node)) return;
      if (!node.hasAttribute("tabindex")) node.setAttribute("tabindex", "0");
      if (!node.hasAttribute("role")) node.setAttribute("role", "button");
    });
  }

  function activate(target) {
    const text = target.getAttribute("data-copy");
    if (text == null) return;
    copyText(text).then(() => {
      showToast("copied");
    }).catch(() => {
      showToast("copy failed");
    });
  }

  function init(root) {
    const targetRoot = root || document;
    if (!targetRoot || initializedRoots.has(targetRoot)) return;
    initializedRoots.add(targetRoot);
    enhanceTargets(targetRoot);
    targetRoot.addEventListener("click", (event) => {
      const target = findCopyTarget(event.target);
      if (!target) return;
      event.preventDefault();
      activate(target);
    });
    targetRoot.addEventListener("keydown", (event) => {
      if (event.key !== "Enter") return;
      const target = findCopyTarget(event.target);
      if (!target) return;
      event.preventDefault();
      activate(target);
    });
  }

  window.StriatumCopyOnClick = {
    init,
    // Exposed for static contract tests and dev console inspection.
    _allowedContainerSelectors: ALLOWED_COPY_CONTAINER_SELECTORS,
  };
}());
