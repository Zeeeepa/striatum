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

  function clamp(value, min, max) {
    return Math.max(min, Math.min(max, value));
  }

  function enableGraphViewport(graph) {
    const svg = graph.querySelector("svg");
    if (!svg) return;
    const controls = graph.parentElement ? graph.parentElement.querySelector("[data-graph-controls]") : null;
    const viewBox = svg.getAttribute("viewBox");
    const viewBoxParts = viewBox ? viewBox.split(/\s+/).map(Number) : [];
    const baseWidth = Number(svg.getAttribute("width")) || viewBoxParts[2] || svg.getBoundingClientRect().width || 1;
    const baseHeight = Number(svg.getAttribute("height")) || viewBoxParts[3] || svg.getBoundingClientRect().height || 1;
    let scale = 1;

    function applyScale(nextScale) {
      scale = clamp(nextScale, 0.4, 2.5);
      svg.style.width = `${Math.round(baseWidth * scale)}px`;
      svg.style.height = `${Math.round(baseHeight * scale)}px`;
      graph.dataset.graphZoom = scale.toFixed(2);
    }

    function fitScale() {
      const available = Math.max(1, graph.clientWidth - 16);
      return clamp(available / baseWidth, 0.4, 2.5);
    }

    if (controls) {
      controls.addEventListener("click", (event) => {
        const button = event.target.closest("[data-graph-zoom]");
        if (!button) return;
        const action = button.dataset.graphZoom;
        if (action === "in") applyScale(scale * 1.2);
        if (action === "out") applyScale(scale / 1.2);
        if (action === "fit") applyScale(fitScale());
        if (action === "reset") applyScale(1);
      });
    }

    let drag = null;
    graph.addEventListener("pointerdown", (event) => {
      if (event.button !== 0 || event.target.closest(".graph-node-link")) return;
      drag = {
        pointerId: event.pointerId,
        x: event.clientX,
        y: event.clientY,
        left: graph.scrollLeft,
        top: graph.scrollTop,
      };
      graph.classList.add("is-panning");
      graph.setPointerCapture(event.pointerId);
    });
    graph.addEventListener("pointermove", (event) => {
      if (!drag || drag.pointerId !== event.pointerId) return;
      graph.scrollLeft = drag.left - (event.clientX - drag.x);
      graph.scrollTop = drag.top - (event.clientY - drag.y);
    });
    graph.addEventListener("pointerup", (event) => {
      if (!drag || drag.pointerId !== event.pointerId) return;
      drag = null;
      graph.classList.remove("is-panning");
      graph.releasePointerCapture(event.pointerId);
    });
    graph.addEventListener("pointercancel", () => {
      drag = null;
      graph.classList.remove("is-panning");
    });
    graph.addEventListener("keydown", (event) => {
      if (event.key === "+" || event.key === "=") {
        event.preventDefault();
        applyScale(scale * 1.2);
      } else if (event.key === "-" || event.key === "_") {
        event.preventDefault();
        applyScale(scale / 1.2);
      } else if (event.key === "0") {
        event.preventDefault();
        applyScale(1);
      } else if (event.key.toLowerCase() === "f") {
        event.preventDefault();
        applyScale(fitScale());
      } else if (event.key === "ArrowLeft") {
        event.preventDefault();
        graph.scrollLeft -= 32;
      } else if (event.key === "ArrowRight") {
        event.preventDefault();
        graph.scrollLeft += 32;
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        graph.scrollTop -= 32;
      } else if (event.key === "ArrowDown") {
        event.preventDefault();
        graph.scrollTop += 32;
      }
    });
    applyScale(1);
  }

  document.addEventListener("DOMContentLoaded", () => {
    const graph = document.querySelector(".graph-container");
    if (!graph) return;
    enableGraphViewport(graph);
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
