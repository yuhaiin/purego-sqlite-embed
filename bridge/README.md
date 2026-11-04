# Optional row reader, ABI v1

`row.c` is compiled alongside the unmodified official SQLite amalgamation, into
the same shared library. It exports this C ABI (cdecl on Windows):

```c
int purego_sqlite_read_row_v1(sqlite3_stmt *stmt, void *out,
                            int32_t count, uint32_t stride);
```

The caller must hold its connection lock and have a live statement positioned
on SQLITE_ROW. `count` must equal its current column count, `stride` must be 24,
and `out` must point to `count` writable records. Each record has native byte
order, with these fixed offsets on both 32-bit and 64-bit targets:

| Offset | C type | Meaning |
| --- | --- | --- |
| 0 | uint64_t | INTEGER modulo 2^64, or FLOAT IEEE 754 double bits |
| 8 | uint64_t | Zero-extended native TEXT/BLOB address |
| 16 | int32_t | TEXT/BLOB byte length |
| 20 | int32_t | Actual SQLite storage class (1..5) |

Unused fields are zeroed. An empty BLOB may have address zero and length zero;
NULL has storage class 5. Strings are byte-counted, including embedded NULs.
The reader does not step, reset, clear, finalize, allocate an output buffer,
retain caller memory or call back into Go. SQLite can allocate during ordinary
column conversion; errors are returned as SQLite result codes, including NOMEM.
Output is valid only after SQLITE_OK. Native payloads must be copied before the
next step/reset/finalize. The caller must not retry a failed read through a
different implementation.

The bridge uses only public SQLite APIs. ELF builds bind internal function
references locally to prevent another loaded SQLite from interposing on the
bridge's engine. A driver resolves this symbol from the same handle as all its
SQLite APIs. Missing symbols select the ordinary API path, so older prebuilts
and system libraries remain usable. Future incompatible layouts use a new
symbol suffix; v1 is never reinterpreted.
