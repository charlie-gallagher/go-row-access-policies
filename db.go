package main

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

type AccessDB interface {
	Close() error
	ListTables() ([]string, error)
	Setup() error
	Exec(stmt string, args ...any) error
	Prepare(query string) (*sql.Stmt, error)
	SelectOne(query string, args ...any) (map[string]any, error)
	Select(query string, args ...any) ([]map[string]any, error)
}

var ErrTooManyRows = errors.New("expected 1 row")

// Default instance of AccessDb is a SqliteDB
type SqliteDB struct {
	handle *sql.DB
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
	if _, err := db.handle.Exec(`
	create table if not exists policies(role varchar, control_column varchar, value varchar);
	delete from policies;
	create table if not exists roles(role varchar unique);
	delete from roles;`); err != nil {
		return err
	}
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
	return all_rows[0], nil
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

	return out, nil
}

func (db *SqliteDB) getRows(rows *sql.Rows, minRows, maxRows int) ([]map[string]any, error) {
	// Populate an N-slice with the values
	columns, err := rows.Columns()
	if err != nil {
		return []map[string]any{}, err
	}

	n_rows := 0
	out := []map[string]any{}
	for rows.Next() {
		n_rows++

		// Check if we've exceeded the maximum number of rows
		if n_rows > maxRows {
			return []map[string]any{}, fmt.Errorf("%w: expected at most %d rows, got %d", ErrTooManyRows, maxRows, n_rows)
		}

		// Populate an N-slice using slice of pointers to any
		row := make([]any, len(columns))
		rowPtrs := make([]any, len(columns))
		for i := range row {
			rowPtrs[i] = &row[i]
		}
		if err := rows.Scan(rowPtrs...); err != nil {
			return []map[string]any{}, err
		}

		// Construct a map of column name to value
		rowMap := make(map[string]any)
		for i, col := range columns {
			value := row[i]

			if b, ok := value.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = value
			}
		}
		out = append(out, rowMap)
	}

	if n_rows == 0 && minRows > 0 {
		return []map[string]any{}, fmt.Errorf("%w: expected at least %d rows, got %d", sql.ErrNoRows, minRows, n_rows)
	}

	// Check if we've satisfied the minimum number of rows
	if n_rows < minRows {
		return []map[string]any{}, fmt.Errorf("expected at least %d rows, got %d", minRows, n_rows)
	}

	return out, nil
}

func (db *SqliteDB) Prepare(query string) (*sql.Stmt, error) {
	return db.handle.Prepare(query)
}
