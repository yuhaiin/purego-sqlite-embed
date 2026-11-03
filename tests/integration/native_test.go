package integration

import (
	"bytes"
	"database/sql"
	"math"
	"path/filepath"
	"testing"

	sqlite "github.com/yuhaiin/purego-sqlite"
	sqliteembed "github.com/yuhaiin/purego-sqlite-embed"
)

func TestPrebuiltNativeSQLite(t *testing.T) {
	path, err := sqliteembed.Path()
	if err != nil {
		t.Fatal(err)
	}
	info, err := sqliteembed.Info()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", info)
	connector, err := sqlite.NewConnector(filepath.Join(t.TempDir(), "prebuilt.db"), sqlite.WithLibrary(path), sqlite.WithWAL())
	if err != nil {
		t.Fatal(err)
	}
	if connector.Version() != sqliteembed.SQLiteVersion {
		t.Fatal("unexpected engine", connector.Version())
	}
	for _, feature := range []string{"fts5", "json", "jsonb", "rtree", "math", "dbstat"} {
		ok, err := connector.HasFeature(feature)
		if err != nil || !ok {
			t.Fatal("required native feature", feature, ok, err)
		}
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE data(i INTEGER,r REAL,t TEXT,b BLOB)"); err != nil {
		t.Fatal(err)
	}
	blob := bytes.Repeat([]byte{0, 255, 128, 7}, 32768)
	for _, integer := range []int64{math.MinInt64, math.MaxInt64} {
		if _, err := db.Exec("INSERT INTO data VALUES(?,?,?,?)", integer, math.Pi, "你好\x00SQLite", blob); err != nil {
			t.Fatal(err)
		}
		var gotInt int64
		var gotDouble float64
		var text string
		var gotBlob []byte
		if err := db.QueryRow("SELECT i,r,t,b FROM data WHERE i=?", integer).Scan(&gotInt, &gotDouble, &text, &gotBlob); err != nil {
			t.Fatal(err)
		}
		if gotInt != integer || gotDouble != math.Pi || text != "你好\x00SQLite" || !bytes.Equal(gotBlob, blob) {
			t.Fatal("ABI/value round trip failed")
		}
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
}
