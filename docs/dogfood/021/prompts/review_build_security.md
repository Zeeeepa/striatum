Verify: API key never leaks (search source for any path that
logs/serializes/returns the key value); sanitizer enforced
(no <script> in rendered output); path traversal refused;
chat unconfigured produces clean error not leak; scratch
JSONL written under .striatum/ only.
