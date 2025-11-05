package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"

	_ "modernc.org/sqlite"
)

type AccessDB interface {
	Close() error
	ListTables() ([]string, error)
	Setup() error
	Begin() (*sql.Tx, error)
	Prepare(query string) (*sql.Stmt, error)
	Exec(stmt string, args ...any) error
	SelectOne(query string, args ...any) (map[string]any, error)
	Select(query string, args ...any) ([]map[string]any, error)
}

var ErrTooManyRows = errors.New("expected 1 row")

// Default instance of AccessDb is a SqliteDB
type SqliteDB struct {
	handle *sql.DB
}

type AccessResult struct {
	Data    []map[string]any
	Columns []string
}

// SqliteDB implements the AccessDB interface
var _ AccessDB = &SqliteDB{}

// Create a new SqliteDB instance
//
// The connect string is passed to the sql.Open function to create a new database
// connection. It is either a file path or a ":memory:" string to create an
// in-memory database.
//
// Returns a new SqliteDB instance and an error if the database connection
// fails. Automatically pings the database to ensure it is connected.
func NewSqliteDB(connect string) (SqliteDB, error) {
	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite", connect)
	if err != nil {
		return SqliteDB{}, err
	}

	if err = db.Ping(); err != nil {
		return SqliteDB{}, err
	}

	sqlite_db := SqliteDB{handle: db}

	return sqlite_db, nil
}

func (db *SqliteDB) Close() error {
	return db.handle.Close()
}

func (db *SqliteDB) ListTables() ([]string, error) {
	var output []string
	rows, err := db.Select("select name from sqlite_master where type = 'table' order by name")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		output = append(output, row["name"].(string))
	}
	return output, nil
}

func (db *SqliteDB) Setup() error {
	tx, err := db.handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
	create table if not exists policies(role varchar, control_column varchar, value varchar);
	delete from policies;
	create table if not exists roles(role varchar unique);
	delete from roles;`); err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (db *SqliteDB) Exec(stmt string, args ...any) error {
	if _, err := db.handle.Exec(stmt, args...); err != nil {
		return err
	}
	return nil
}

func (db *SqliteDB) SelectOne(query string, args ...any) (map[string]any, error) {
	rows, err := db.handle.Query(query, args...)
	if err != nil {
		return map[string]any{}, err
	}
	defer rows.Close()

	all_rows, err := db.getRows(rows, 1, 1)
	if err != nil {
		return map[string]any{}, err
	}
	return all_rows.Data[0], nil
}

func (db *SqliteDB) Select(query string, args ...any) ([]map[string]any, error) {
	rows, err := db.handle.Query(query, args...)
	if err != nil {
		return []map[string]any{}, err
	}
	defer rows.Close()

	out, err := db.getRows(rows, 0, 1000)
	if err != nil {
		return []map[string]any{}, err
	}

	return out.Data, nil
}

func (db *SqliteDB) getRows(rows *sql.Rows, minRows, maxRows int) (*AccessResult, error) {
	// Get columns and their types
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	column_types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	// Get the types of the columns
	types := make([]reflect.Type, len(column_types))
	for i, tp := range column_types {
		st := tp.ScanType()
		if st == nil {
			log.Printf("warning: ScanType is null for column %q", tp.Name())
			continue
		}
		types[i] = st
	}

	n_rows := 0
	out := []map[string]any{}
	for rows.Next() {
		n_rows++

		// Check if we've exceeded the maximum number of rows
		if n_rows > maxRows {
			return nil, fmt.Errorf("%w: expected at most %d rows, got %d", ErrTooManyRows, maxRows, n_rows)
		}

		// Populate an N-slice using slice of pointers to the correct interface type
		row_ptrs := make([]any, len(column_types))
		for i := range row_ptrs {
			row_ptrs[i] = reflect.New(types[i]).Interface()
		}
		err = rows.Scan(row_ptrs...)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		// The values are easier to work with as values, not pointers
		row_values := make([]any, len(column_types))
		for i := range row_values {
			row_values[i] = reflect.ValueOf(row_ptrs[i]).Elem().Interface()
		}

		// Construct a map of column name to value
		rowMap := make(map[string]any)
		for i, col := range columns {
			rowMap[col] = row_values[i]
		}
		out = append(out, rowMap)
	}

	if n_rows == 0 && minRows > 0 {
		return nil, fmt.Errorf("%w: expected at least %d rows, got %d", sql.ErrNoRows, minRows, n_rows)
	}

	// Check if we've satisfied the minimum number of rows
	if n_rows < minRows {
		return nil, fmt.Errorf("expected at least %d rows, got %d", minRows, n_rows)
	}

	return &AccessResult{Data: out, Columns: columns}, nil
}

func (db *SqliteDB) Prepare(query string) (*sql.Stmt, error) {
	return db.handle.Prepare(query)
}

func (db *SqliteDB) Begin() (*sql.Tx, error) {
	return db.handle.Begin()
}
