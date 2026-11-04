// Package sqliteembed embeds prebuilt official SQLite shared libraries. Path
// verifies and extracts the target library for use with sqlite.WithLibrary.
// It has no driver dependency, requires no CGO itself, and never downloads code.
package sqliteembed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
)

var ErrUnsupportedPlatform = errors.New("sqliteembed: no prebuilt library for this target")

// Metadata describes the selected binary and its verified official source.
type Metadata struct {
	SQLiteVersion   string   `json:"sqlite_version"`
	RowBridgeABI    uint32   `json:"row_bridge_abi"`
	RowBridgeSHA256 string   `json:"row_bridge_sha256"`
	GOOS            string   `json:"goos"`
	GOARCH          string   `json:"goarch"`
	Libc            string   `json:"libc"`
	FloatABI        string   `json:"float_abi"`
	MinimumOS       string   `json:"minimum_os"`
	Filename        string   `json:"filename"`
	SHA256          string   `json:"sha256"`
	Size            int64    `json:"size"`
	SourceURL       string   `json:"source_url"`
	SourceSHA256    string   `json:"source_sha256"`
	SourceSHA3      string   `json:"source_sha3_256"`
	Compiler        string   `json:"compiler"`
	Flags           []string `json:"flags"`
	BuildCommit     string   `json:"build_commit"`
	BuildRun        string   `json:"build_run"`
}

type asset struct{ data, manifest []byte }

// Exactly one generated target file sets selected during package initialization.
var selected asset

var metadata = sync.OnceValues(func() (Metadata, error) { return inspect(selected) })
var libraryPath = sync.OnceValues(func() (string, error) {
	info, err := metadata()
	if err != nil {
		return "", err
	}
	return extract(selected.data, info, "")
})

// Info returns a copy of the selected asset's metadata after verifying its hash.
// Linux defaults to glibc; use -tags sqlite_musl for a musl target. MIPS floating
// point selection follows Go's GOMIPS feature tags.
func Info() (Metadata, error) {
	info, err := metadata()
	info.Flags = slices.Clone(info.Flags)
	return info, err
}

// Path returns an absolute path to the embedded native library. Extraction is
// private and synchronized, and the path remains valid for process life. There
// is deliberately no cleanup/unload API: SQLite connections may still depend on
// the file, especially on Windows. Remove stale directories only after exit.
func Path() (string, error) { return libraryPath() }

func inspect(a asset) (Metadata, error) {
	if len(a.data) == 0 || len(a.manifest) == 0 {
		return Metadata{}, fmt.Errorf("%w: %s/%s (check libc and floating-point build tags)", ErrUnsupportedPlatform, runtime.GOOS, runtime.GOARCH)
	}
	var info Metadata
	if err := json.Unmarshal(a.manifest, &info); err != nil {
		return Metadata{}, fmt.Errorf("sqliteembed: invalid manifest: %w", err)
	}
	if info.SQLiteVersion != SQLiteVersion || info.GOOS != runtime.GOOS || info.GOARCH != runtime.GOARCH {
		return Metadata{}, errors.New("sqliteembed: manifest version/platform mismatch")
	}
	expected := "libsqlite3.so"
	if runtime.GOOS == "windows" {
		expected = "sqlite3.dll"
	}
	if runtime.GOOS == "darwin" {
		expected = "libsqlite3.dylib"
	}
	if info.Filename != expected {
		return Metadata{}, errors.New("sqliteembed: invalid native library filename")
	}
	if info.Size != int64(len(a.data)) {
		return Metadata{}, errors.New("sqliteembed: embedded library size mismatch")
	}
	sum := sha256.Sum256(a.data)
	if hex.EncodeToString(sum[:]) != info.SHA256 {
		return Metadata{}, errors.New("sqliteembed: embedded library SHA-256 mismatch")
	}
	return info, nil
}

// parent is empty in production (os.TempDir). Tests provide an isolated parent.
func extract(data []byte, info Metadata, parent string) (string, error) {
	dir, err := os.MkdirTemp(parent, "purego-sqlite-embed-")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, info.Filename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0500)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	success = true
	return path, nil
}
