# Item 3: Service Split Cleanup

Move one stable non-SQLite route/rendering boundary out of `service.py` while
preserving compatibility wrappers and tests. Do not change service authority
or fallback policy.
