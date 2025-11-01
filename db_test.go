package main

import (
	"slices"
	"testing"
)

func TestNewSqliteWorks(t *testing.T) {
	db, err := NewSqliteDB(":memory:")
	if err != nil {
		t.Fatalf("Could not create new SqliteDB: %v", err)
	}
	defer db.handle.Close()
	// check ping works
	if err = db.handle.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestSqliteCloseWorks(t *testing.T) {
	db, err := NewSqliteDB(":memory:")
	if err != nil {
		t.Fatalf("Could not create new SqliteDB: %v", err)
	}
	// Test ping before and after close
	if err = db.handle.Ping(); err != nil {
		t.Fatalf("ping unexpectedly failed: %v", err)
	}
	db.Close()
	if err = db.handle.Ping(); err == nil {
		t.Errorf("ping unexpectedly succeeded: %v", err)
	}
}

func TestSqliteListTablesWorks(t *testing.T) {
	db, err := NewSqliteDB(":memory:")
	if err != nil {
		t.Fatalf("Could not create new SqliteDB: %v", err)
	}

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

// func TestSqliteInitWorks(t *testing.T) {
// 	db, err := NewSqliteDB(":memory:")
// 	if err != nil {
// 		t.Fatalf("Could not create new SqliteDB: %v", err)
// 	}
// 	// Init should create
// }
