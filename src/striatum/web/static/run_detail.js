(function () {
  "use strict";

  function tooltipText(node) {
    const title = node.dataset.jobId || "Job";
    const role = node.dataset.roleId || "-";
    const state = node.dataset.state || "-";
    const start = node.dataset.startedAt || node.dataset.createdAt || "";
    const end = node.dataset.completedAt || "";
    const duration = start ? window.StriatumUI.formatDurationBetween(start, end) : "-";
    return `${title}\nRole: ${role}\nState: ${state}\nDuration: ${duration}`;
  }

  function positionTooltip(tip, x, y) {
    const margin = 12;
    tip.style.left = "0";
    tip.style.top = "0";
    const rect = tip.getBoundingClientRect();
    const left = Math.min(x + margin, window.innerWidth - rect.width - margin);
    const top = Math.min(y + margin, window.innerHeight - rect.height - margin);
    tip.style.left = `${Math.max(margin, left)}px`;
    tip.style.top = `${Math.max(margin, top)}px`;
  }

  document.addEventListener("DOMContentLoaded", () => {
    const graph = document.querySelector(".graph-container");
    if (!graph) return;
    const tip = document.createElement("div");
    tip.className = "graph-tooltip";
    tip.hidden = true;
    document.body.appendChild(tip);

    const nodes = Array.from(graph.querySelectorAll(".graph-node"));
    nodes.forEach((node) => {
      const link = node.closest(".graph-node-link");
      const focusTarget = link || node;
      focusTarget.setAttribute("tabindex", focusTarget.getAttribute("tabindex") || "0");
      const show = (event) => {
        tip.textContent = tooltipText(node);
        tip.hidden = false;
        positionTooltip(tip, event.clientX || window.innerWidth / 2, event.clientY || window.innerHeight / 2);
      };
      const move = (event) => {
        if (!tip.hidden) positionTooltip(tip, event.clientX, event.clientY);
      };
      const hide = () => {
        tip.hidden = true;
      };
      focusTarget.addEventListener("mouseenter", show);
      focusTarget.addEventListener("mousemove", move);
      focusTarget.addEventListener("mouseleave", hide);
      focusTarget.addEventListener("focus", show);
      focusTarget.addEventListener("blur", hide);
    });
  });
}());
