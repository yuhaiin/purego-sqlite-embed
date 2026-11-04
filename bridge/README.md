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

# Optional scalar executor, ABI v1

`exec.c` is compiled into the same library and exports:

```c
int purego_sqlite_exec_scalar_v1(sqlite3_stmt *stmt, void *frame,
                               int32_t count, uint32_t stride);
```

The caller holds the connection lock and provides a live prepared statement
ready for a complete bind and execution. `count` equals the statement's parameter
count and `stride` is 16. `frame` contains a 16-byte writable result followed by
`count` 16-byte inputs, all with native byte order on 32-bit and 64-bit targets:

| Record | Offset 0 (uint64_t) | Offset 8 (uint64_t) |
| --- | --- | --- |
| Result | Last insert row ID, modulo 2^64 | Affected rows |
| Each input | Integer modulo 2^64 or IEEE double bits | SQLite type: INTEGER=1, FLOAT=2, NULL=5 |

The driver passes SQLite-allocated memory containing scalar bits only, with no
Go pointers. All input types are validated before any bind. Parameters bind in
index order. The helper drains all result rows, captures the row ID and changes
(zero for read-only statements), and resets the statement on success. It returns
SQLITE_DONE after a successful reset. Scalar bindings may remain, but the next
execution must fully rebind every slot. No callback or caller buffer is retained.

On any failure, result output is invalid. The caller must capture the connection
error **before** reset/clear and must never retry the execution. The helper leaves
binding/step errors untouched for that purpose. Cancellation still uses the
driver's connection lifecycle and sqlite3_interrupt. TEXT/BLOB, named argument
mapping and Query stay on the ordinary driver path. Missing symbols select that
path automatically. `HasFeature("exec_scalar_v1")` reports availability.
