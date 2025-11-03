package main

import (
	"database/sql"
	"errors"
	"slices"
	"testing"
)

func TestNewSqliteWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")
	defer db.Close()
	// check ping works
	if err := db.handle.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestSqliteCloseWorks(t *testing.T) {
	db := getNewSqliteDB(t, ":memory:")
	defer db.Close()

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
	defer db.Close()

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
	defer db.Close()

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
	db := getRawInitializedDbHandle(t)
	defer db.Close()

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
	defer db.Close()

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

func TestSqliteSelectOneWorks(t *testing.T) {
	db := getSqliteDBWithData(t, ":memory:")
	defer db.Close()

	// Query for data and inspect result
	want_map := map[string]string{
		"role":           "admin",
		"control_column": "Region",
		"value":          "Southern",
	}
	got_map, err := db.SelectOne("select role, control_column, value from policies where role = ?", "admin")
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"role", "control_column", "value"}
	for _, key := range keys {
		want := want_map[key]
		got, ok := got_map[key]
		if !ok {
			t.Errorf("Expected column %s but didn't find it", key)
		} else if got != want {
			t.Errorf("wanted: %s, got: %s", want, got)
		}
	}
}

func TestSqliteSelectOneThrowsForNoRows(t *testing.T) {
	db := getSqliteDBWithData(t, ":memory:")
	defer db.Close()

	// Query for data and inspect result
	_, err := db.SelectOne("select role, control_column, value from policies where role = ?", "not_a_role")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Error("expected no rows error")
	}
}

func TestSqliteSelectOneThrowsForTwoRows(t *testing.T) {
	db := getSqliteDBWithData(t, ":memory:")
	defer db.Close()

	// Query for data and inspect result
	_, err := db.SelectOne("select role, control_column, value from policies where control_column = ?", "Region")
	if !errors.Is(err, ErrTooManyRows) {
		t.Error("expected too many rows error")
	}
}

func TestSqliteSelectWorksForNoRows(t *testing.T) {
	db := getSetupSqliteDB(t, ":memory:")
	defer db.Close()

	got_result, err := db.Select("select role, control_column, value from policies")
	if err != nil {
		t.Fatal(err)
	}
	if len(got_result) != 0 {
		t.Errorf("expected no rows, got %v", got_result)
	}
}

func TestSqliteSelectWorksForOneRow(t *testing.T) {
	db := getSqliteDBWithData(t, ":memory:")
	defer db.Close()

	// Query for data and inspect result
	want_result := []map[string]string{{
		"role":           "admin",
		"control_column": "Region",
		"value":          "Southern",
	}}
	got_result, err := db.Select("select role, control_column, value from policies where role = ?", "admin")
	if err != nil {
		t.Fatal(err)
	}
	want_map := want_result[0]
	got_map := got_result[0]
	keys := []string{"role", "control_column", "value"}
	for _, key := range keys {
		want := want_map[key]
		got, ok := got_map[key]
		if !ok {
			t.Errorf("Expected column %s but didn't find it", key)
		} else if got != want {
			t.Errorf("wanted: %s, got: %s", want, got)
		}
	}
}

func TestSqliteSelectWorksForRows(t *testing.T) {
	db := getSqliteDBWithData(t, ":memory:")
	defer db.Close()

	// Query for data and inspect result
	want_result := []map[string]string{
		{
			"role":           "admin",
			"control_column": "Region",
			"value":          "Southern",
		},
		{
			"role":           "employee",
			"control_column": "Region",
			"value":          "Eastern",
		},
	}
	got_result, err := db.Select("select role, control_column, value from policies where control_column = ?", "Region")
	if err != nil {
		t.Fatal(err)
	}

	if len(got_result) != 2 {
		t.Fatalf("Want length 2, got %d", len(got_result))
	}

	for i := range want_result {
		want_map := want_result[i]
		got_map := got_result[i]
		keys := []string{"role", "control_column", "value"}
		for _, key := range keys {
			want := want_map[key]
			got, ok := got_map[key]
			if !ok {
				t.Errorf("Expected column %s but didn't find it", key)
			} else if got != want {
				t.Errorf("wanted: %s, got: %s", want, got)
			}
		}
	}
}

func TestSqlitePreparedStatementWorks(t *testing.T) {
	db := getSetupSqliteDB(t, ":memory:")
	defer db.Close()

	// Execute bulk statement
	stmt, err := db.Prepare("insert into roles (role) values (?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()

	roles := []string{"a", "b", "c"}
	for _, role := range roles {
		if _, err := stmt.Exec(role); err != nil {
			t.Fatal(err)
		}
	}

	// Confirm data was written
	result, err := db.Select("select role from roles order by role")
	if err != nil {
		t.Fatal(err)
	}
	got_roles := []string{}
	for _, v := range result {
		got_roles = append(got_roles, v["role"].(string))
	}
	if !slices.Equal(roles, got_roles) {
		t.Errorf("want: %v, got: %v", roles, got_roles)
	}
}

func TestSqliteBeginCommits(t *testing.T) {
	db := getSetupSqliteDB(t, ":memory:")
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec("insert into roles (role) values (?)", "new_role"); err != nil {
		t.Fatalf("exec: %v", err)
	}

	if err = tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	result, err := db.Select("select role from roles")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 rows, got %d", len(result))
	}
}

func TestSqliteBeginRollsBack(t *testing.T) {
	db := getSetupSqliteDB(t, ":memory:")
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec("insert into roles (role) values (?)", "new_role"); err != nil {
		t.Fatalf("exec: %v", err)
	}

	if err = tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	result, err := db.Select("select role from roles")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result))
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

// Get a SqliteDB with some data in it
//
// Creates a new, initialized SqliteDB and adds two rows to to the policies table.
// There is one row for the admin role and one row for the employee role. Both
// rows apply to the Region control column.
func getSqliteDBWithData(t *testing.T, connect string) SqliteDB {
	t.Helper()
	db := getSetupSqliteDB(t, connect)
	if err := db.Exec(
		`insert into policies (role, control_column, value) values (?, ?, ?);`,
		"admin", "Region", "Southern",
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		`insert into policies (role, control_column, value) values (?, ?, ?);`,
		"employee", "Region", "Eastern",
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func getRawInitializedDbHandle(t *testing.T) *sql.DB {
	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`
	create table if not exists policies(role varchar, control_column varchar, value varchar);
	delete from policies;
	create table if not exists roles(role varchar unique);
	delete from roles;`); err != nil {
		t.Fatal(err)
	}

	return db
}
