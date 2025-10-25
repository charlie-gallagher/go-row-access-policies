package main

import (
	"database/sql"
	"fmt"
	"log"
	"testing"
)

func TestPolicyItemConvertsToJson(t *testing.T) {

	tests := []struct {
		input  PolicyItem
		output string
	}{
		// NOTE: it might not be best practice to serialize different inputs to
		// the same output, but it's convenient for the user
		{PolicyItem{}, "null"},
		{PolicyItem{Column: "charlie", Values: []string{}}, "null"},
		{PolicyItem{Column: "charlie", Values: []string{"one", "two", "three"}}, `{"column":"charlie","values":["one","two","three"]}`},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("input: %v, output: %s", test.input, test.output), func(t *testing.T) {
			got := test.input.ToJson()
			if got != test.output {
				t.Fail()
			}
		})
	}
}

func TestPolicyConvertsToJson(t *testing.T) {
	tests := []struct {
		input  Policy
		output string
	}{
		{Policy{}, "null"},
		{
			Policy{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two", "three"}}}},
			`{"role":"admin","policy":[{"column":"Region","values":["one","two","three"]}]}`,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("input: %v, output: %s", test.input, test.output), func(t *testing.T) {
			got := test.input.ToJson()
			if got != test.output {
				t.Fail()
			}
		})
	}
}

func TestValidateConfigWorks(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		err := ValidateConfig([]byte(
			`{"policies":[{"role":"admin", "policy":[{"column":"Region", "values":["one","two"]}]}]}`,
		))
		if err != nil {
			t.Fail()
		}
	})
	t.Run("Valid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/valid_policy_set.json")
		if err != nil {
			t.Fail()
		}
	})

	t.Run("Empty set of policies is ok", func(t *testing.T) {
		err := ValidateConfig([]byte(`{"policies":[]}`))
		if err != nil {
			t.Fail()
		}
	})
}

func TestValidateConfigFails(t *testing.T) {
	t.Run("Invalid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/invalid_policy_set.json")
		if err == nil {
			t.Fail()
		}
	})
	t.Run("Invalid config", func(t *testing.T) {
		err := ValidateConfig([]byte(
			`{"policies":[{"oops":"admin", "policy_items":[{"column":"Region", "values":["one","two"]}]}]}`,
		))
		if err == nil {
			t.Fail()
		}
	})
}

func TestDbInitWorks(t *testing.T) {
	t.Run("InitDb works", func(t *testing.T) {
		db, err := getDbHandle()
		if err != nil {
			log.Printf("Error getting db handle: %v\n", err)
			t.Fail()
		}

		if err = InitDb(db); err != nil {
			log.Printf("Error initializing db: %v\n", err)
			t.Fail()
		}
		db.Close()
	})

	t.Run("policies table is created", func(t *testing.T) {
		db, err := getInitializedDbHandle()
		if err != nil {
			log.Printf("Error getting initialized db handle: %v\n", err)
			t.Fail()
		}
		var tableName string
		if err = fetchOneRow(db, "select name from sqlite_master where type = 'table'", &tableName); err != nil {
			log.Printf("Error fetching one row: %v\n", err)
			t.Fail()
		}
		if tableName != "policies" {
			log.Printf("Table name is not policies: %s\n", tableName)
			t.Fail()
		}
		db.Close()
	})
}

func getInitializedDbHandle() (*sql.DB, error) {
	db, err := getDbHandle()
	if err != nil {
		return nil, err
	}
	if err = InitDb(db); err != nil {
		return nil, err
	}
	return db, nil
}

func getDbHandle() (*sql.DB, error) {
	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func fetchOneRow(db *sql.DB, query string, dest ...any) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		return fmt.Errorf("no rows found")
	}

	return rows.Scan(dest...)
}
