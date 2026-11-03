# purego-sqlite-embed

Prebuilt official SQLite shared libraries embedded in Go applications. Companion to [purego-sqlite](https://github.com/yuhaiin/purego-sqlite); this module has no driver dependency, no CGO, and no runtime downloads. Requires Go 1.26 or newer.

Only the library matching the target OS, architecture, libc and floating-point ABI is embedded in the executable. The Go module download contains all committed assets. SQLite is a native C engine, loaded dynamically after extraction.

## Use

```sh
go get github.com/yuhaiin/purego-sqlite github.com/yuhaiin/purego-sqlite-embed
```

```go
package main

import (
    "database/sql"
    "log"

    sqlite "github.com/yuhaiin/purego-sqlite"
    sqliteembed "github.com/yuhaiin/purego-sqlite-embed"
)

func main() {
    library, err := sqliteembed.Path()
    if err != nil { log.Fatal(err) }
    connector, err := sqlite.NewConnector("file:app.db", sqlite.WithLibrary(library), sqlite.WithWAL())
    if err != nil { log.Fatal(err) }
    db := sql.OpenDB(connector)
    defer db.Close()
    var version string
    if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil { log.Fatal(err) }
    log.Println(version)
}
```

On Linux, macOS and Windows amd64/arm64, build with `CGO_ENABLED=0`. The library still depends on the target system's native C runtime. On MIPS and FreeBSD, the driver automatically uses its CGO dynamic-loader fallback: enable CGO and provide a matching C cross compiler/sysroot when cross compiling. SQLite headers and a separately installed SQLite library are unnecessary.

## Targets

SQLite version: **3.54.1**. Features include FTS5, JSON/JSONB, RTree, dbstat and math functions, with thread safety and extension loading enabled. The driver controls permission to load extensions.

| Target | Baseline | Selection |
| --- | --- | --- |
| Linux amd64, arm64 | glibc 2.17 | default |
| Linux amd64, arm64 | musl 1.2.x | `-tags sqlite_musl` |
| Linux mipsle | MIPS32r2, glibc 2.17 | `GOMIPS=hardfloat` or `softfloat` |
| Linux mipsle | MIPS32r2, musl 1.2.x | `GOMIPS=hardfloat` or `softfloat`, `-tags sqlite_musl` |
| macOS amd64, arm64 | macOS 11.0 | default |
| Windows amd64 | Windows 10 | default |
| Windows arm64 | Windows 10 ARM64 | default |
| FreeBSD amd64 | FreeBSD 14.3 | default |

Go does not detect the target libc. Select `sqlite_musl` explicitly for Alpine/OpenWrt musl builds; for a musl program using the driver, enable CGO and use `-tags sqlite_cgo,sqlite_musl` with a matching musl compiler. `GOMIPS` must agree with the C compiler, sysroot and firmware ABI. Do not load a glibc library on musl or mix MIPS floating-point ABIs. Other targets return `ErrUnsupportedPlatform`; Android and iOS are excluded explicitly.

## Extraction and metadata

`Path()` verifies SHA-256, extracts once into a unique private directory under `os.TempDir()`, and returns the absolute library path. Concurrent callers share one result. Unix directory/file modes are 0700/0500. Set `TMPDIR` (or the Windows temporary directory) to a writable location that allows loading executable code; `noexec` filesystems may prevent loading.

The library and its directory remain until explicitly removed after process exit. There is no cleanup or unload API because live connections can retain native code pointers and Windows locks loaded DLLs. Long-running services should arrange stale-directory cleanup after exit.

`Info()` returns verified metadata: source checksums, binary checksum, compiler, flags, ABI, minimum system version, and build workflow URL. These hashes detect corrupted assets; they do not replace trust in reviewed repository contents and the Go module checksum database. Native code is not sandboxed.

## Build and verify

`scripts/source.json` pins the official amalgamation URL, SHA-256 and the SHA3-256 published by SQLite. Builds check both hashes. `scripts/targets.json` defines target ABIs and build constraints. Linux/Windows use verified Zig 0.17.0; macOS and FreeBSD use native toolchains. Manifests record toolchain details. Builds are source-pinned; different host toolchains may produce different binary hashes.

```sh
python3 scripts/install-zig.py # adds Zig to GITHUB_PATH in Actions; locally add printed install path to PATH
python3 scripts/build.py linux_amd64_glibc
python3 scripts/install.py .build/dist
CGO_ENABLED=0 SQLITE_EMBED_REQUIRE=1 go test ./...
(cd tests/integration && CGO_ENABLED=0 go test ./...)
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/verify.py # requires all committed target assets
```

Runtime CI covers glibc and musl Linux amd64/arm64, macOS amd64/arm64, Windows amd64/arm64, FreeBSD amd64 and QEMU glibc MIPS hardfloat. Other MIPS ABI variants are cross compiled and their ELF floating-point ABI and Go selection are checked; validate them on matching firmware before deployment.

Integration tests use a separate module so the embedding module retains zero dependencies. They load the extracted binary through the driver, exercise required SQLite capabilities, persist WAL databases, round-trip 64-bit integers/floating-point/Unicode/NUL/blob values and check database integrity.

## Automatic updates

[Update SQLite](.github/workflows/update.yml) checks the official stable-release metadata daily. A new release updates the source pin, rebuilds all targets, validates candidates on native Linux/macOS/Windows/FreeBSD runners plus musl containers, then opens or updates a single PR using `peter-evans/create-pull-request@v8`. It never merges automatically. Manual dispatch supports rebuilding the pinned version and disabling PR publication. Feature-branch pushes that change build inputs also build candidates without publication. [Check updater App](.github/workflows/check-app.yml) supports a quick manual credential check.

The updater follows [kitte's GitHub App setup](https://github.com/yuhaiin/kitte/blob/main/.github/workflows/update.yml):

- Actions variable `APP_ID`: the App ID/client ID (currently 1142630).
- Actions secret `PRIVATE_KEY`: the App's PEM private key. GitHub does not allow reading/copying an existing repository secret through its API; configure it on this repository or share an organization secret with this repository.
- Install the App for this repository with Contents and Pull requests write permissions. The workflow requests a short-lived token scoped to this repository and these permissions.

Build/test jobs need no App secret. App credentials are used only after all candidates pass. App-generated PRs trigger ordinary CI, including when organization policy disables PR creation with the default `GITHUB_TOKEN`.

Original Go code is MIT licensed. SQLite is public domain; bundled toolchain notices are in [NOTICE](NOTICE) and [third_party](third_party).
