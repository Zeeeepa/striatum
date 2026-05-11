# Implementer Role (Dogfood 031)

You implement only the design scope accepted by the fresh design reviews. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual runner behavior.

Do not ship a daemon that claims more guarantees than the accepted plan defends with code and tests. Mutation capabilities must default off. Direct CLI mode must remain a working fallback unless the accepted plan explicitly retires it and migrates the affected tests, examples, and docs. The daemon must not parse stdout/stderr as workflow state and must not capture transcripts by default.
