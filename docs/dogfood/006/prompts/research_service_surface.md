# Research Service Surface

Read the work packet first. Include the exact author: line near the top
of the artifact you publish.

Survey:
- src/striatum/api.py - the api.invoke surface; argv shape, response
  envelope, exit-code semantics.
- src/striatum/mcp.py - existing local stdio JSON-RPC wrapper; the
  patterns it uses for tool dispatch.
- src/striatum/cli/parser.py - argparse structure; how to enumerate
  mutation vs read commands.
- src/striatum/dashboard.py - existing TUI; what JSON shape it consumes.
- The events table schema and existing read patterns.
- Python stdlib options: http.server, socketserver, ThreadingHTTPServer.
- POSIX Unix-domain socket binding via socket.AF_UNIX + http.server's
  HTTPServer subclass.

Verify RFC 0012's claims line by line:
- Does api.invoke really return the documented envelope shape?
- What CLI commands mutate state vs read state? (For the
  --allow-mutations gate.)
- Does mcp.py have a mutation/read classification we can reuse?
- Are events table reads safe from a separate connection?

Publish docs/dogfood/006/research/SERVICE_SURFACE.md with:
- exact source citations (file:line);
- recommended stdlib server class and why;
- proposed mutation-detection rule (whitelist of read verbs vs
  blacklist of write verbs);
- SSE poll cadence and event shape;
- risks / unknowns;
- minimum-touch implementation order.
