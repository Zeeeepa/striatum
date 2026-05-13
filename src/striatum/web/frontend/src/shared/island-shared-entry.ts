/**
 * Production entry for the `island-shared` Rollup bundle.
 *
 * The shared entry is loaded once from `base.html` so every page warms the
 * shared CSS (and any Rollup-extracted common chunks) before per-page island
 * scripts run. It MUST NOT mount any island: `src/main.ts` is the dev-shell
 * entry that mounts everything, and shipping that file as `island-shared.js`
 * would re-mount every island on every page (RFC 0038 V1.5 F3).
 */
import "./theme.css";
export {};
