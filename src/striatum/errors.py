"""Domain errors for the striatum CLI."""

from __future__ import annotations

# Stable exit-code table. Codes 11 and 12 are reserved by RFC 0043 §3 for
# daemon-required CLI refusals; the V1 RFC 0028 daemon module reserved 11/12
# for auth/capability refusals, which have been renumbered to 14/15 to free
# the RFC 0043 codes. Code 10 remains RFC 0030's version-skew refusal.
EXIT_GENERIC_FAILURE = 1
EXIT_PARSE = 2
EXIT_NOT_FOUND = 3
EXIT_INVALID_TRANSITION = 4
EXIT_LEASE = 5
EXIT_ARTIFACT = 6
EXIT_BRANCH_CONFIRMATION = 7
EXIT_WORKFLOW = 8
EXIT_SCHEMA_VERSION = 9
EXIT_RPC_VERSION_INCOMPATIBLE = 10
EXIT_DAEMON_UNREACHABLE = 11
EXIT_REPO_NOT_MIGRATED = 12
EXIT_DAEMON_REGISTRY = 13
EXIT_DAEMON_AUTH = 14
EXIT_DAEMON_CAPABILITY = 15


class StriatumError(RuntimeError):
    """Raised for expected CLI failures with stable exit codes."""

    def __init__(self, message: str, *, exit_code: int = 1) -> None:
        super().__init__(message)
        self.exit_code = exit_code


class NotFoundError(StriatumError):
    """Raised when a referenced run, session, job, message, or artifact is missing."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_NOT_FOUND)


class InvalidTransitionError(StriatumError):
    """Raised when a command would violate the state machine."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_INVALID_TRANSITION)


class LeaseError(StriatumError):
    """Raised for stale lease or ownership mismatches."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_LEASE)


class ArtifactError(StriatumError):
    """Raised for artifact and write-scope violations."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_ARTIFACT)


class BranchConfirmationError(StriatumError):
    """Raised when work is requested before branch confirmation."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_BRANCH_CONFIRMATION)


class WorkflowError(StriatumError):
    """Raised when workflow JSON is invalid.

    RFC 0024 V2: ``field_path`` carries an optional dotted/indexed
    path to the offending field (e.g. ``jobs[2].role_id``,
    ``cycles[0].max_iterations``) so the visual builder can highlight
    inline. ``None`` means raise sites have not been updated to carry
    a path; consumers fall back to the message banner.
    """

    def __init__(self, message: str, *, field_path: str | None = None) -> None:
        super().__init__(message, exit_code=EXIT_WORKFLOW)
        self.field_path = field_path


class SchemaVersionError(StriatumError):
    """Raised when the local state schema is incompatible with this install."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_SCHEMA_VERSION)


class DaemonUnreachableError(StriatumError):
    """Raised when the daemon socket cannot be reached (RFC 0043 §3).

    ``hint`` carries the structured remediation summary used by the JSON
    error envelope; the stderr template includes a multi-line block with
    Linux systemd, macOS launchd, foreground, and Postgres install hints.
    """

    def __init__(self, message: str, *, hint: str | None = None) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_UNREACHABLE)
        self.hint = hint


class RepoNotMigratedError(StriatumError):
    """Raised when a CLI verb targets a repo with no daemon migration row (RFC 0043 §3).

    ``hint`` carries the structured registration remediation string used by
    the JSON error envelope.
    """

    def __init__(self, message: str, *, hint: str | None = None) -> None:
        super().__init__(message, exit_code=EXIT_REPO_NOT_MIGRATED)
        self.hint = hint
