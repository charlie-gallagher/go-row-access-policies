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
	t.Run("Valid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/valid_policy_set.json")
		if err != nil {
			t.Fail()
		}
	})

	valid_policy_set_tests := []string{
		`{"policies":[]}`,
		`{"policies":[{"role":"admin", "policy":[]}]}`,
		`{"policies":[{"role":"admin", "policy":[{"column":"Region", "values":[]}]}]}`,
		`{"policies":[{"role":"admin", "policy":[{"column":"Region", "values":["one","two"]}]}]}`,
	}
	for _, test := range valid_policy_set_tests {
		t.Run(fmt.Sprintf("valid policy set: %s", test), func(t *testing.T) {
			err := ValidateConfig([]byte(test))
			if err != nil {
				log.Printf("Error validating policy set: %v\n", err)
				t.Fail()
			}
		})
	}
}

func TestValidateConfigFails(t *testing.T) {
	t.Run("Invalid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/invalid_policy_set.json")
		if err == nil {
			t.Fail()
		}
	})

	invalid_policy_set_tests := []string{
		// Mising role key
		`{"policies":[{"oops":"admin", "policy":[{"column":"Region", "values":["one","two"]}]}]}`,
		// Mising policy key
		`{"policies":[{"role":"admin", "policy_items":[{"column":"Region", "values":["one","two"]}]}]}`,
		// Just a role (not a policy set)
		`{"role":"admin", "policy":[{"column":"Region", "values":["Eastern"]}]}`,
	}
	for _, test := range invalid_policy_set_tests {
		t.Run(fmt.Sprintf("invalid policy set: %s", test), func(t *testing.T) {
			err := ValidateConfig([]byte(test))
			if err == nil {
				t.Fail()
			}
		})
	}
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
		if err = fetchOneRow(db, "select name from sqlite_master where type = 'table' and name = 'policies'", &tableName); err != nil {
			log.Printf("Error fetching one row: %v\n", err)
			t.Fail()
		}
		db.Close()
	})

	t.Run("roles table is created", func(t *testing.T) {
		db, err := getInitializedDbHandle()
		if err != nil {
			log.Printf("Error getting initialized db handle: %v\n", err)
			t.Fail()
		}
		var tableName string
		if err = fetchOneRow(db, "select name from sqlite_master where type = 'table' and name = 'roles'", &tableName); err != nil {
			log.Printf("Error fetching one row: %v\n", err)
			t.Fail()
		}
		db.Close()
	})
}

func TestDbLoadWorks(t *testing.T) {

	policies := []struct {
		input  Policy
		output string
	}{
		{
			Policy{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two", "three"}}}},
			`{"role":"admin","policy":[{"column":"Region","values":["one","two","three"]}]}`,
		},
		{
			Policy{Role: "east_mgr", Policy: []PolicyItem{{Column: "State", Values: []string{"__all__"}}}},
			`null`,
		},
		{
			Policy{Role: "north_mgr", Policy: []PolicyItem{
				{Column: "Region", Values: []string{"Northern", "Eastern"}},
				{Column: "State", Values: []string{"WA", "OR", "CA", "ID", "NV"}},
			}},
			`{"role":"north_mgr","policy":[{"column":"Region","values":["Northern","Eastern"]},{"column":"State","values":["WA","OR","CA","ID","NV"]}]}`,
		},
		{
			Policy{Role: "north_mgr", Policy: []PolicyItem{
				{Column: "Region", Values: []string{"Northern", "Eastern"}},
				{Column: "State", Values: []string{"__all__"}},
			}},
			`{"role":"north_mgr","policy":[{"column":"Region","values":["Northern","Eastern"]}]}`,
		},
	}

	for _, test := range policies {
		t.Run(fmt.Sprintf("db load: %s", test.output), func(t *testing.T) {
			db, err := getInitializedDbHandle()
			if err != nil {
				log.Printf("Error getting initialized db handle: %v\n", err)
				t.Fail()
			}
			if err = LoadDbWithPolicies(db, &PolicySet{Policies: []Policy{test.input}}); err != nil {
				log.Printf("Error loading db with policies: %v\n", err)
				t.Fail()
			}
			fetch, err := GetPolicy(db, test.input.Role)
			if err != nil {
				log.Printf("Error getting policy: %v\n", err)
				t.Fail()
			}
			if fetch.ToJson() != test.output {
				log.Printf("Policy mismatch: got %s, want %s\n", fetch.ToJson(), test.output)
				t.Fail()
			}
			db.Close()
		})
	}
}

func TestGetPolicyWorks(t *testing.T) {
	db, err := getInitializedDbHandle()
	if err != nil {
		log.Printf("Error getting initialized db handle: %v\n", err)
		t.Fail()
	}
	policy_set, err := LoadRolePolicies("testdata/valid_policy_set.json")
	if err != nil {
		log.Printf("Error loading role policies: %v\n", err)
		t.Fail()
	}
	if err = LoadDbWithPolicies(db, policy_set); err != nil {
		log.Printf("Error loading db with policies: %v\n", err)
		t.Fail()
	}

	policy, err := GetPolicy(db, "admin")
	if err != nil {
		log.Printf("Error getting policy: %v\n", err)
		t.Fail()
	}
	if policy.ToJson() != `null` {
		log.Printf("Policy mismatch: got %s, want %s\n", policy.ToJson(), `null`)
		t.Fail()
	}
	db.Close()
}

func TestGetPolicyFailsIfRoleDoesNotExist(t *testing.T) {
	db, err := getInitializedDbHandle()
	if err != nil {
		log.Printf("Error getting initialized db handle: %v\n", err)
		t.Fail()
	}
	policy_set, err := LoadRolePolicies("testdata/valid_policy_set.json")
	if err != nil {
		log.Printf("Error loading role policies: %v\n", err)
		t.Fail()
	}
	if err = LoadDbWithPolicies(db, policy_set); err != nil {
		log.Printf("Error loading db with policies: %v\n", err)
		t.Fail()
	}

	_, err = GetPolicy(db, "does_not_exist")
	if err == nil {
		log.Printf("Expected error getting policy for role that does not exist, but got none")
		t.Fail()
	}
	db.Close()
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

	out := rows.Scan(dest...)
	if rows.Next() {
		return fmt.Errorf("Found too many rows (expected 1, got >1)")
	}
	return out
}
