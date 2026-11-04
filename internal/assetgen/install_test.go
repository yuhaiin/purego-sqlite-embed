package assetgen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuhaiin/purego-sqlite-embed/internal/sqlitesource"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root, source := t.TempDir(), t.TempDir()
	// Use real manifests and binaries, so integrity and bridge pins are exercised.
	for _, name := range []string{"scripts/targets.json", "bridge/row.c", "bridge/exec.c"} {
		b, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := sqlitesource.WriteFile(filepath.Join(root, name), b); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"manifest.json", "libsqlite3.so"} {
		b, err := os.ReadFile(filepath.Join("../../assets/linux_amd64_glibc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := sqlitesource.WriteFile(filepath.Join(source, "linux_amd64_glibc", name), b); err != nil {
			t.Fatal(err)
		}
	}
	// Candidate jobs install only their platform before running tests. During a
	// version update, this committed Linux fixture may still be the old version.
	// Give the isolated fixture its own matching pin rather than the job's pin.
	b, err := os.ReadFile(filepath.Join(source, "linux_amd64_glibc/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	pin := sqlitesource.Pin{Version: m.Version, URL: m.SourceURL, SHA256: m.SourceSHA256, SHA3: m.SourceSHA3}
	if err := sqlitesource.WriteJSON(filepath.Join(root, "scripts/source.json"), pin); err != nil {
		t.Fatal(err)
	}
	return root, source
}

func TestInstallVerifiedCandidateAndRegenerateOffline(t *testing.T) {
	root, source := fixture(t)
	if err := Install(root, source, nil); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(root, "asset_linux_amd64_glibc.go")
	b, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("//go:embed assets/linux_amd64_glibc/libsqlite3.so")) || !bytes.Contains(b, []byte("//go:build linux && !android && amd64 && !sqlite_musl")) {
		t.Fatal("incorrect embed selection")
	}
	before, err := os.Stat(generated)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(root, filepath.Join(root, "assets"), nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("identical generated source was rewritten")
	}
}

func TestInvalidCandidateDoesNotPublishAnyFiles(t *testing.T) {
	for _, kind := range []string{"binary", "source-version", "helper", "unknown-target"} {
		t.Run(kind, func(t *testing.T) {
			root, source := fixture(t)
			manifestPath := filepath.Join(source, "linux_amd64_glibc/manifest.json")
			b, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			var keys []string
			switch kind {
			case "binary":
				if err := os.WriteFile(filepath.Join(source, "linux_amd64_glibc/libsqlite3.so"), []byte("tampered"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "source-version":
				m["sqlite_version"] = "99.0.0"
			case "helper":
				m["exec_bridge_sha256"] = "wrong"
			case "unknown-target":
				keys = []string{"../../unknown"}
			}
			if err := sqlitesource.WriteJSON(manifestPath, m); err != nil {
				t.Fatal(err)
			}
			if err := Install(root, source, keys); err == nil {
				t.Fatal("invalid candidate accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "version.go")); !os.IsNotExist(err) {
				t.Fatal("published a version before validation")
			}
			if _, err := os.Stat(filepath.Join(root, "assets")); !os.IsNotExist(err) {
				t.Fatal("published a binary before validation")
			}
		})
	}
}
