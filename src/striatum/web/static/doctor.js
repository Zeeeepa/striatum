(function () {
  "use strict";

  const HIDE_KEY = "striatum.ui.doctor.hide_terminal";
  const GROUPS_KEY = "striatum.ui.doctor.groups";
  const TERMINAL_STATES = ["completed", "failed", "canceled"];

  function readHideDefault(problemCount) {
    try {
      const parsed = JSON.parse(window.localStorage.getItem(HIDE_KEY) || "");
      if (parsed && parsed.version === 1 && ["on", "off"].includes(parsed.value)) {
        return parsed.value === "on";
      }
    } catch (_err) {
      // Fall through.
    }
    return problemCount > 20;
  }

  function writeHide(value) {
    try {
      window.localStorage.setItem(HIDE_KEY, JSON.stringify({ version: 1, value: value ? "on" : "off" }));
    } catch (_err) {
      // Filtering remains usable without persistence.
    }
  }

  function readGroupPrefs() {
    try {
      const parsed = JSON.parse(window.localStorage.getItem(GROUPS_KEY) || "");
      if (!parsed || parsed.version !== 1) throw new Error("bad version");
      return {
        collapsed: Array.isArray(parsed.collapsed) ? parsed.collapsed : [],
        expanded: Array.isArray(parsed.expanded) ? parsed.expanded : [],
      };
    } catch (_err) {
      return { collapsed: [], expanded: [] };
    }
  }

  function writeGroupPrefs(groups) {
    try {
      window.localStorage.setItem(GROUPS_KEY, JSON.stringify({ version: 1, ...groups }));
    } catch (_err) {
      // Non-fatal.
    }
  }

  document.addEventListener("DOMContentLoaded", () => {
    const root = document.querySelector(".doctor-groups");
    if (!root) return;
    const problemCount = Number(root.dataset.problemCount || "0");
    const hideToggle = document.querySelector("#doctor-hide-terminal");
    const filteredEmpty = document.querySelector("#doctor-filtered-empty");
    const groups = Array.from(document.querySelectorAll(".doctor-group"));
    let groupPrefs = readGroupPrefs();

    groups.forEach((group) => {
      const check = group.dataset.check || "";
      if (groupPrefs.collapsed.includes(check)) group.open = false;
      if (groupPrefs.expanded.includes(check)) group.open = true;
      group.addEventListener("toggle", () => {
        const collapsed = new Set(groupPrefs.collapsed);
        const expanded = new Set(groupPrefs.expanded);
        if (group.open) {
          expanded.add(check);
          collapsed.delete(check);
        } else {
          collapsed.add(check);
          expanded.delete(check);
        }
        groupPrefs = { collapsed: Array.from(collapsed), expanded: Array.from(expanded) };
        writeGroupPrefs(groupPrefs);
      });
    });

    const apply = () => {
      const hideTerminal = Boolean(hideToggle.checked);
      let visibleProblems = 0;
      groups.forEach((group) => {
        let visibleInGroup = 0;
        group.querySelectorAll(".problem-list > li").forEach((item) => {
          const runState = item.dataset.runState || "";
          const isTerminal = item.dataset.terminalRun === "true" || TERMINAL_STATES.includes(runState);
          const show = !(hideTerminal && isTerminal);
          item.hidden = !show;
          if (show) visibleInGroup += 1;
        });
        group.hidden = visibleInGroup === 0;
        visibleProblems += visibleInGroup;
      });
      if (filteredEmpty) filteredEmpty.hidden = visibleProblems !== 0;
      writeHide(hideTerminal);
    };

    hideToggle.checked = readHideDefault(problemCount);
    hideToggle.addEventListener("change", apply);
    apply();
  });
}());
