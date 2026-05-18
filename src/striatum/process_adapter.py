"""Process-adapter neutral helpers and legacy compatibility wrappers."""

from __future__ import annotations

import string
from collections.abc import Mapping
from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

PROCESS_SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS process_executions (
  process_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  lease_id TEXT NOT NULL REFERENCES leases(lease_id),
  packet_id TEXT NOT NULL REFERENCES work_packets(packet_id),
  adapter TEXT NOT NULL,
  command_json TEXT NOT NULL,
  cwd TEXT NOT NULL,
  scratch_path TEXT NOT NULL,
  stdin_mode TEXT NOT NULL CHECK (stdin_mode IN ('packet','none')),
  stdio_mode TEXT NOT NULL CHECK (stdio_mode IN ('suppressed','inherit')),
  pid INTEGER,
  state TEXT NOT NULL CHECK (state IN (
    'starting','running','exited','failed','timed_out','lost'
  )),
  exit_code INTEGER,
  started_at TEXT NOT NULL,
  ended_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_process_executions_run_job
  ON process_executions(run_id, job_id, started_at);
"""

__all__ = [
    "PROCESS_SCHEMA_SQL",
    "_expand_lane_env_values",
    "command_array",
    "ensure_process_schema",
    "lane_config",
    "mark_process_exited",
    "mark_process_failed",
    "mark_process_lost",
    "mark_process_running",
    "mark_process_timed_out",
    "prepare_process_launch",
    "run_process_adapter",
    "string_mapping",
]


def _expand_lane_env_values(
    raw_env: Mapping[str, str], expansion: Mapping[str, str]
) -> dict[str, str]:
    """Expand ``${VAR}`` / ``$VAR`` references in lane env values."""
    return {
        key: string.Template(value).safe_substitute(expansion)
        for key, value in raw_env.items()
    }


def run_process_adapter(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_process_adapter().run_process_adapter(*args, **kwargs),
    )


def ensure_process_schema(*args: Any, **kwargs: Any) -> None:
    _legacy_process_adapter().ensure_process_schema(*args, **kwargs)


def prepare_process_launch(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_process_adapter().prepare_process_launch(*args, **kwargs),
    )


def mark_process_running(*args: Any, **kwargs: Any) -> None:
    _legacy_process_adapter().mark_process_running(*args, **kwargs)


def mark_process_failed(*args: Any, **kwargs: Any) -> None:
    _legacy_process_adapter().mark_process_failed(*args, **kwargs)


def mark_process_exited(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_process_adapter().mark_process_exited(*args, **kwargs),
    )


def mark_process_timed_out(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_process_adapter().mark_process_timed_out(*args, **kwargs),
    )


def mark_process_lost(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_process_adapter().mark_process_lost(*args, **kwargs))


def lane_config(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_process_adapter().lane_config(*args, **kwargs))


def command_array(*args: Any, **kwargs: Any) -> list[str]:
    return cast(list[str], _legacy_process_adapter().command_array(*args, **kwargs))


def string_mapping(*args: Any, **kwargs: Any) -> bool:
    return bool(_legacy_process_adapter().string_mapping(*args, **kwargs))


def _legacy_process_adapter() -> Any:
    return import_module("striatum.legacy_sqlite.process_adapter")
