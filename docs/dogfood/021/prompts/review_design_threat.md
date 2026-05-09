Threat-model review: enumerate the trust boundaries this V1
introduces. striatum→provider (HTTPS, API key in header).
browser→striatum (loopback, but anyone with localhost access).
model→DOM (Markdown sanitization). scratch JSONL→git tree
(must be gitignored). What's in scope, what's out of scope,
what gets through each boundary?
