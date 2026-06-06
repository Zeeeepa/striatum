# Author Role — RFC 0114 read-scope successor

You author and finalize the successor design RFC to RFC 0113 R1 (GH #164),
expanding runtime read-scope least privilege to the `principals` /
`principal_clients` and/or `client_sessions` surfaces.

This is DESIGN ONLY. You produce an RFC document. You do not change `go/`
source, you do not apply any owner bundle to a live database, and you do not
restart the daemon. Implementation is a later PR.

Produce the expected artifact at the path declared in your work packet and stay
inside the declared write scope (only the dogfood artifact root). Read the
required context docs first; verify your claims against the cited source files.
