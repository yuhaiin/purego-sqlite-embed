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
	fastRows, err := connector.HasFeature("row_read_v1")
	if err != nil || !fastRows || info.RowBridgeABI != 1 {
		t.Fatalf("required row bridge absent: feature=%v ABI=%d err=%v", fastRows, info.RowBridgeABI, err)
	}
	fastExec, err := connector.HasFeature("exec_scalar_v1")
	if err != nil || !fastExec || info.ExecBridgeABI != 1 {
		t.Fatalf("required scalar executor absent: feature=%v ABI=%d err=%v", fastExec, info.ExecBridgeABI, err)
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
	// Reuse a scalar statement across success, constraint failure and recovery.
	if _, err := db.Exec("CREATE TABLE scalar(id INTEGER PRIMARY KEY, value REAL, flag, optional)"); err != nil {
		t.Fatal(err)
	}
	s, err := db.Prepare("INSERT INTO scalar VALUES(?,?,?,?) RETURNING id")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, integer := range []int64{math.MinInt64, math.MaxInt64} {
		r, err := s.Exec(integer, math.Pi, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		if id, _ := r.LastInsertId(); id != integer {
			t.Fatal("scalar row ID", id)
		}
		if n, _ := r.RowsAffected(); n != 1 {
			t.Fatal("scalar changes", n)
		}
	}
	if _, err := s.Exec(int64(1), 0.5, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exec(int64(1), 1.5, false, nil); err == nil {
		t.Fatal("constraint failure discarded")
	}
	if _, err := s.Exec(int64(2), 2.5, false, nil); err != nil {
		t.Fatal("failed statement not reusable", err)
	}
	var scalarCount int
	if err := db.QueryRow("SELECT count(*) FROM scalar").Scan(&scalarCount); err != nil || scalarCount != 4 {
		t.Fatal("scalar execution duplicated or lost writes", scalarCount, err)
	}
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
}
