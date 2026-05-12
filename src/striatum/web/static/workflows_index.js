(function () {
  "use strict";

  const STORAGE_KEY = "striatum.ui.workflows_index.filter";
  const STATUSES = ["all", "valid", "invalid"];

  function readFilter() {
    try {
      const parsed = JSON.parse(window.localStorage.getItem(STORAGE_KEY) || "");
      if (!parsed || parsed.version !== 1) throw new Error("bad version");
      return {
        search: typeof parsed.search === "string" ? parsed.search : "",
        status: STATUSES.includes(parsed.status) ? parsed.status : "all",
      };
    } catch (_err) {
      return { search: "", status: "all" };
    }
  }

  function writeFilter(filter) {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ version: 1, ...filter }));
    } catch (_err) {
      // Filtering remains usable without persistence.
    }
  }

  function rowMatches(row, filter) {
    const haystack = `${row.dataset.path || ""} ${row.dataset.workflowId || ""}`.toLowerCase();
    const searchMatches = !filter.search || haystack.includes(filter.search.toLowerCase());
    const statusMatches = filter.status === "all" || (row.dataset.status || "") === filter.status;
    return searchMatches && statusMatches;
  }

  document.addEventListener("DOMContentLoaded", () => {
    const form = document.querySelector("#workflows-index-filter");
    if (!form) return;
    const rows = Array.from(document.querySelectorAll(".workflows-table tbody tr"));
    const searchInput = document.querySelector("#workflow-search");
    const statusButtons = Array.from(document.querySelectorAll("[data-status-filter]"));
    const filteredEmpty = document.querySelector("#workflow-filtered-empty");
    let filter = readFilter();

    const syncControls = () => {
      searchInput.value = filter.search;
      statusButtons.forEach((button) => {
        button.setAttribute("aria-pressed", button.dataset.statusFilter === filter.status ? "true" : "false");
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
    };

    searchInput.addEventListener("input", () => {
      filter = { ...filter, search: searchInput.value };
      apply();
    });
    document.querySelector("#workflow-search-clear").addEventListener("click", () => {
      filter = { ...filter, search: "" };
      syncControls();
      apply();
    });
    statusButtons.forEach((button) => {
      button.addEventListener("click", () => {
        filter = { ...filter, status: button.dataset.statusFilter || "all" };
        syncControls();
        apply();
      });
    });
    document.querySelector("#workflow-filter-clear").addEventListener("click", () => {
      filter = { search: "", status: "all" };
      syncControls();
      apply();
    });

    syncControls();
    apply();
  });
}());
