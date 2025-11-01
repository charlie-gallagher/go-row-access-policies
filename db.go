package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type AccessDB interface {
	Init() error
	Close() error
	Exec(db *sql.DB, stmt string, args ...any) error
	Select(db *sql.DB, query string, args ...any) (any, error)
	SelectOne(db *sql.DB, query string, dest any, args ...any) error
}

// Default instance of AccessDb is a SqliteDB
type SqliteDB struct {
	handle *sql.DB
}

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
	rows, err := db.handle.Query("select name from sqlite_master where type = 'table'")
	if err != nil {
		return nil, err
	}
	var name string
	for rows.Next() {
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error scanning db, %v", err)
		}
		output = append(output, name)
	}
	return output, nil
}
