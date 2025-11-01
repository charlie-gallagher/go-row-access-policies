package main

import (
	"slices"
	"testing"
)

func TestNewSqliteWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")
	defer db.handle.Close()
	// check ping works
	if err := db.handle.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestSqliteCloseWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")
	// Test ping before and after close
	if err := db.handle.Ping(); err != nil {
		t.Fatalf("ping unexpectedly failed: %v", err)
	}
	db.Close()
	if err := db.handle.Ping(); err == nil {
		t.Errorf("ping unexpectedly succeeded: %v", err)
	}
}

func TestSqliteListTablesWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")

	// Create a table manually
	if _, err := db.handle.Exec("create table if not exists policies(role varchar, control_column varchar, value varchar);"); err != nil {
		t.Fatalf("failed to create new table: %v", err)
	}

	// Check list of tables for that table name
	table_list, err := db.ListTables()
	if err != nil {
		t.Fatalf("failed to list tables, got %v", err)
	}
	if !slices.Contains(table_list, "policies") {
		t.Errorf("did not find table 'policies' in list of tables")
	}
}

func TestSqliteSetupWorks(t *testing.T) {
	db := getSetupSqliteDB(t, ":memory:")

	// Assert that the tables exist
	tables, err := db.ListTables()
	if err != nil {
		t.Fatalf("failed to list system tables: %v", err)
	}
	expected_tables := []string{"policies", "roles"}
	for _, want := range expected_tables {
		if !slices.Contains(tables, want) {
			t.Errorf("%s not found among system tables", want)
		}
	}
}

func TestSqliteSetupTruncatesExistingTables(t *testing.T) {
	// Create table ahead of time and add some rows to it
	db := getInitializedDbHandle(t)
	if _, err := db.Exec(
		`insert into policies (role, control_column, value) values (?, ?, ?);`,
		"admin", "Region", "Southern",
	); err != nil {
		t.Fatal(err)
	}

	// Run setup anyway
	sqlite_db := SqliteDB{handle: db}
	if err := sqlite_db.Setup(); err != nil {
		t.Fatalf("failed to (re)create system tables: %v", err)
	}

	// Confirm that there are no rows in 'policies'
	rows, err := sqlite_db.handle.Query("select * from policies")
	if err != nil {
		t.Fatalf("failed to query policies: %v", err)
	}
	if rows.Next() {
		t.Errorf("expected no rows in policies")
	}
	rows.Close()
}

func TestSqliteExecWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")

	// Create new table
	if err := db.Exec("create table if not exists test_table(test_column varchar)"); err != nil {
		t.Fatal(err)
	}

	// Confirm that the table is now in the list of tables
	tables, err := db.ListTables()
	if err != nil {
		t.Fatalf("failed to list system tables: %v", err)
	}
	expected_tables := []string{"test_table"}
	for _, want := range expected_tables {
		if !slices.Contains(tables, want) {
			t.Errorf("%s not found among system tables", want)
		}
	}
}

func getNewSqliteDB(t *testing.T, connect string) SqliteDB {
	t.Helper()
	db, err := NewSqliteDB(connect)
	if err != nil {
		t.Fatalf("could not create new SqliteDB: %v", err)
	}
	return db
}

func getSetupSqliteDB(t *testing.T, connect string) SqliteDB {
	t.Helper()
	db := getNewSqliteDB(t, connect)
	if err := db.Setup(); err != nil {
		t.Fatalf("could not set up db: %v", err)
	}
	return db
}
