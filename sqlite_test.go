package sqliteembed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func fixture(t *testing.T) asset {
	t.Helper()
	data := []byte("test native library content")
	digest := sha256.Sum256(data)
	name := "libsqlite3.so"
	if runtime.GOOS == "darwin" {
		name = "libsqlite3.dylib"
	}
	if runtime.GOOS == "windows" {
		name = "sqlite3.dll"
	}
	info := Metadata{SQLiteVersion: SQLiteVersion, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Filename: name, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
	manifest, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return asset{data: data, manifest: manifest}
}
func TestIntegrityAndManifest(t *testing.T) {
	a := fixture(t)
	if _, err := inspect(a); err != nil {
		t.Fatal(err)
	}
	changed := asset{data: bytes.Clone(a.data), manifest: a.manifest}
	changed.data[0] ^= 1
	if _, err := inspect(changed); err == nil {
		t.Fatal("accepted corrupted native code")
	}
	for _, edit := range []func(*Metadata){
		func(m *Metadata) { m.Filename = "../../outside.so" },
		func(m *Metadata) { m.GOARCH = "not-this-architecture" },
		func(m *Metadata) { m.SQLiteVersion = "0.0.0" },
		func(m *Metadata) { m.Size++ },
	} {
		var m Metadata
		if err := json.Unmarshal(a.manifest, &m); err != nil {
			t.Fatal(err)
		}
		edit(&m)
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := inspect(asset{data: a.data, manifest: data}); err == nil {
			t.Fatal("accepted invalid manifest")
		}
	}
	if _, err := inspect(asset{}); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatal(err)
	}
	if _, err := inspect(asset{data: a.data, manifest: []byte("invalid")}); err == nil {
		t.Fatal("accepted malformed manifest")
	}
}
func TestExtraction(t *testing.T) {
	a := fixture(t)
	m, err := inspect(a)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	path, err := extract(a.data, m, parent)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Dir(path)) != parent {
		t.Fatal(path)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, a.data) {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		dir, err := os.Stat(filepath.Dir(path))
		if err != nil || dir.Mode().Perm() != 0700 {
			t.Fatal(dir, err)
		}
		file, err := os.Stat(path)
		if err != nil || file.Mode().Perm() != 0500 {
			t.Fatal(file, err)
		}
	}
	if _, err := extract(a.data, m, filepath.Join(parent, "missing")); err == nil {
		t.Fatal("accepted missing parent")
	}
}
func TestSelectedAssetAndConcurrentPath(t *testing.T) {
	if len(selected.data) == 0 {
		if os.Getenv("SQLITE_EMBED_REQUIRE") == "1" {
			t.Fatal("required prebuilt asset absent")
		}
		t.Skip("target not packaged")
	}
	info, err := Info()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SQLite %s %s/%s %s %s minimum=%s SHA256=%s", info.SQLiteVersion, info.GOOS, info.GOARCH, info.Libc, info.FloatABI, info.MinimumOS, info.SHA256)
	if len(info.Flags) > 0 {
		info.Flags[0] = "changed"
		again, err := Info()
		if err != nil || again.Flags[0] == "changed" {
			t.Fatal("mutable metadata escaped", err)
		}
	}
	paths := make(chan string, 64)
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			path, err := Path()
			if err != nil {
				t.Error(err)
				return
			}
			paths <- path
		})
	}
	wg.Wait()
	close(paths)
	var first string
	for path := range paths {
		if first == "" {
			first = path
		}
		if first != path {
			t.Fatal("duplicate extraction")
		}
	}
	data, err := os.ReadFile(first)
	if err != nil || !bytes.Equal(data, selected.data) {
		t.Fatal("extracted bytes differ", err)
	}
}
