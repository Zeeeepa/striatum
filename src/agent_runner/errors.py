"""Domain errors for the agent_runner CLI."""

from __future__ import annotations


class AgentRunnerError(RuntimeError):
    """Raised for expected CLI failures with stable exit codes."""

    def __init__(self, message: str, *, exit_code: int = 1) -> None:
        super().__init__(message)
        self.exit_code = exit_code


class NotFoundError(AgentRunnerError):
    """Raised when a referenced run, session, job, message, or artifact is missing."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=3)


class InvalidTransitionError(AgentRunnerError):
    """Raised when a command would violate the state machine."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=4)


class LeaseError(AgentRunnerError):
    """Raised for stale lease or ownership mismatches."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=5)


class ArtifactError(AgentRunnerError):
    """Raised for artifact and write-scope violations."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=6)


class BranchConfirmationError(AgentRunnerError):
    """Raised when work is requested before branch confirmation."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=7)


class WorkflowError(AgentRunnerError):
    """Raised when workflow JSON is invalid."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=8)

