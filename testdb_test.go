package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
)

// openTestDB gives each test its own schema in the database named by
// TEST_DATABASE_URL (e.g. postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable),
// and drops it when the test ends. Tests are skipped when it is not set.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	b := make([]byte, 6)
	rand.Read(b)
	schema := "test_" + hex.EncodeToString(b)

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := openDB(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
