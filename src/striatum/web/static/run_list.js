(function () {
  "use strict";

  const STORAGE_KEY = "striatum.ui.run_list.filter";
  const STATES = ["all", "running", "completed", "canceled", "failed", "paused"];
  const DATES = ["all", "24h", "7d", "30d"];

  function readFilter() {
    try {
      const parsed = JSON.parse(window.localStorage.getItem(STORAGE_KEY) || "");
      if (!parsed || parsed.version !== 1) throw new Error("bad version");
      return {
        search: typeof parsed.search === "string" ? parsed.search : "",
        state: STATES.includes(parsed.state) ? parsed.state : "all",
        date: DATES.includes(parsed.date) ? parsed.date : "all",
      };
    } catch (_err) {
      return { search: "", state: "all", date: "all" };
    }
  }

  function writeFilter(filter) {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: 1, ...filter }));
    } catch (_err) {
      // Filtering remains usable without persistence.
    }
  }

  function parseDate(value) {
    if (!value) return null;
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return null;
    return date;
  }

  function dateMatches(createdAt, range) {
    if (range === "all") return true;
    const created = parseDate(createdAt);
    if (!created) return true;
    const days = range === "24h" ? 1 : range === "7d" ? 7 : 30;
    return Date.now() - created.getTime() <= days * 24 * 60 * 60 * 1000;
  }

  function rowMatches(row, filter) {
    const haystack = [
      row.dataset.runId || "",
      row.dataset.branch || "",
      row.dataset.workflowId || "",
    ].join(" ").toLowerCase();
    const searchMatches = !filter.search || haystack.includes(filter.search.toLowerCase());
    const stateMatches = filter.state === "all" || (row.dataset.state || "") === filter.state;
    return searchMatches && stateMatches && dateMatches(row.dataset.createdAt || "", filter.date);
  }

  function durationForRow(row) {
    const state = row.dataset.state || "";
    const completed = row.dataset.completedAt || "";
    const started = row.dataset.startedAt || row.dataset.createdAt || "";
    if (!started) return "-";
    if (completed) return window.StriatumUI.formatDurationBetween(started, completed);
    if (["completed", "failed", "canceled"].includes(state)) return "-";
    return window.StriatumUI.formatDurationBetween(started, "");
  }

  function updateDurations(rows) {
    rows.forEach((row) => {
      const target = row.querySelector(".run-duration");
      if (target) target.textContent = durationForRow(row);
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    const form = document.querySelector("#run-list-filter");
    if (!form) return;
    const rows = Array.from(document.querySelectorAll(".runs-table tbody tr"));
    const searchInput = document.querySelector("#run-search");
    const dateSelect = document.querySelector("#run-date-filter");
    const stateButtons = Array.from(document.querySelectorAll("[data-state-filter]"));
    const filteredEmpty = document.querySelector("#run-filtered-empty");
    let filter = readFilter();

    const syncControls = () => {
      searchInput.value = filter.search;
      dateSelect.value = filter.date;
      stateButtons.forEach((button) => {
        button.setAttribute("aria-pressed", button.dataset.stateFilter === filter.state ? "true" : "false");
      });
    };
    const apply = () => {
      let visible = 0;
      rows.forEach((row) => {
        const show = rowMatches(row, filter);
        row.hidden = !show;
        if (show) visible += 1;
      });
      if (filteredEmpty) filteredEmpty.hidden = visible !== 0;
      writeFilter(filter);
      updateDurations(rows);
    };

    searchInput.addEventListener("input", () => {
      filter = { ...filter, search: searchInput.value };
      apply();
    });
    document.querySelector("#run-search-clear").addEventListener("click", () => {
      filter = { ...filter, search: "" };
      syncControls();
      apply();
    });
    dateSelect.addEventListener("change", () => {
      filter = { ...filter, date: dateSelect.value };
      apply();
    });
    stateButtons.forEach((button) => {
      button.addEventListener("click", () => {
        filter = { ...filter, state: button.dataset.stateFilter || "all" };
        syncControls();
        apply();
      });
    });
    document.querySelector("#run-filter-clear").addEventListener("click", () => {
      filter = { search: "", state: "all", date: "all" };
      syncControls();
      apply();
    });

    syncControls();
    apply();
    window.setInterval(() => updateDurations(rows), 30000);
  });
}());
