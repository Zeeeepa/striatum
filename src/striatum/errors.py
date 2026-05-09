"""Domain errors for the striatum CLI."""

from __future__ import annotations


class StriatumError(RuntimeError):
    """Raised for expected CLI failures with stable exit codes."""

    def __init__(self, message: str, *, exit_code: int = 1) -> None:
        super().__init__(message)
        self.exit_code = exit_code


class NotFoundError(StriatumError):
    """Raised when a referenced run, session, job, message, or artifact is missing."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=3)


class InvalidTransitionError(StriatumError):
    """Raised when a command would violate the state machine."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=4)


class LeaseError(StriatumError):
    """Raised for stale lease or ownership mismatches."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=5)


class ArtifactError(StriatumError):
    """Raised for artifact and write-scope violations."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=6)


class BranchConfirmationError(StriatumError):
    """Raised when work is requested before branch confirmation."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=7)


class WorkflowError(StriatumError):
    """Raised when workflow JSON is invalid.

    RFC 0024 V2: ``field_path`` carries an optional dotted/indexed
    path to the offending field (e.g. ``jobs[2].role_id``,
    ``cycles[0].max_iterations``) so the visual builder can highlight
    inline. ``None`` means raise sites have not been updated to carry
    a path; consumers fall back to the message banner.
    """

    def __init__(self, message: str, *, field_path: str | None = None) -> None:
        super().__init__(message, exit_code=8)
        self.field_path = field_path


class SchemaVersionError(StriatumError):
    """Raised when the local state schema is incompatible with this install."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=9)

