package main

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestPolicyItemConvertsToJson(t *testing.T) {

	tests := map[string]struct {
		input  PolicyItem
		output string
	}{
		// NOTE: it might not be best practice to serialize different inputs to
		// the same output, but it's convenient for the user
		"Empty policy item":                 {PolicyItem{}, "null"},
		"Column with no values":             {PolicyItem{Column: "charlie", Values: []string{}}, "null"},
		"Column with values (typical case)": {PolicyItem{Column: "charlie", Values: []string{"one", "two", "three"}}, `{"column":"charlie","values":["one","two","three"]}`},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := test.input.ToJson()
			if got != test.output {
				t.Errorf("Policy item mismatch: got %s, want %s\n", got, test.output)
			}
		})
	}
}

func TestPolicyConvertsToJson(t *testing.T) {
	// NOTE: __all__ values do not get treated specially in the ToJson() method,
	// so they are not tested here.
	tests := map[string]struct {
		input  Policy
		output string
	}{
		"Empty policy": {Policy{}, "null"},
		"Policy with one item (typical case)": {
			Policy{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two", "three"}}}},
			`{"role":"admin","policy":[{"column":"Region","values":["one","two","three"]}]}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := test.input.ToJson()
			if got != test.output {
				t.Errorf("Policy mismatch: got %s, want %s\n", got, test.output)
			}
		})
	}
}

func TestValidateConfigWorks(t *testing.T) {
	t.Run("Valid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/valid_policy_set.json")
		if err != nil {
			t.Errorf("Error validating config file: %v\n", err)
		}
	})

	valid_policy_set_tests := map[string]string{
		"Empty policy set":                               `{"policies":[]}`,
		"Policy set with one empty policy":               `{"policies":[{"role":"admin", "policy":[]}]}`,
		"Policy set with one policy item with no values": `{"policies":[{"role":"admin", "policy":[{"column":"Region", "values":[]}]}]}`,
		"Policy set with one policy item with values":    `{"policies":[{"role":"admin", "policy":[{"column":"Region", "values":["one","two"]}]}]}`,
	}
	for name, test := range valid_policy_set_tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateConfig([]byte(test))
			if err != nil {
				t.Errorf("Error validating policy set: %v\n", err)
			}
		})
	}
}

func TestValidateConfigFails(t *testing.T) {
	t.Run("Invalid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/invalid_policy_set.json")
		if err == nil {
			t.Errorf("Expected error validating config file, but got none")
		}
	})

	invalid_policy_set_tests := map[string]string{
		"Missing role key":               `{"policies":[{"oops":"admin", "policy":[{"column":"Region", "values":["one","two"]}]}]}`,
		"Missing policy key":             `{"policies":[{"role":"admin", "policy_items":[{"column":"Region", "values":["one","two"]}]}]}`,
		"Just a role (not a policy set)": `{"role":"admin", "policy":[{"column":"Region", "values":["Eastern"]}]}`,
	}
	for name, test := range invalid_policy_set_tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateConfig([]byte(test))
			if err == nil {
				t.Errorf("Expected error validating config, but got none")
			}
		})
	}
}

func TestDbInitWorks(t *testing.T) {
	t.Run("InitDb works", func(t *testing.T) {
		db := getDbHandle(t)
		if err := InitDb(db); err != nil {
			t.Fatalf("Error initializing db: %v\n", err)
		}
	})

	t.Run("policies table is created", func(t *testing.T) {
		db := getInitializedDbHandle(t)
		var tableName string
		fetchOneRow(t, db, "select name from sqlite_master where type = 'table' and name = 'policies'", &tableName)
		db.Close()
	})

	t.Run("roles table is created", func(t *testing.T) {
		db := getInitializedDbHandle(t)
		var tableName string
		fetchOneRow(t, db, "select name from sqlite_master where type = 'table' and name = 'roles'", &tableName)
		db.Close()
	})
}

func TestDbLoadWorks(t *testing.T) {
	// TODO: This test set needs to be broken up. It tests DbLoad, GetPolicy, and ToJson.

	policies := map[string]struct {
		input  Policy
		output string
	}{
		"Policy with one item": {
			Policy{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two", "three"}}}},
			`{"role":"admin","policy":[{"column":"Region","values":["one","two","three"]}]}`,
		},
		"Policy with one __all__ item": {
			Policy{Role: "east_mgr", Policy: []PolicyItem{{Column: "State", Values: []string{"__all__"}}}},
			`null`,
		},
		"Policy with two items": {
			Policy{Role: "north_mgr", Policy: []PolicyItem{
				{Column: "Region", Values: []string{"Northern", "Eastern"}},
				{Column: "State", Values: []string{"WA", "OR", "CA", "ID", "NV"}},
			}},
			`{"role":"north_mgr","policy":[{"column":"Region","values":["Northern","Eastern"]},{"column":"State","values":["WA","OR","CA","ID","NV"]}]}`,
		},
		"Policy with two items, one __all__ item": {
			Policy{Role: "north_mgr", Policy: []PolicyItem{
				{Column: "Region", Values: []string{"Northern", "Eastern"}},
				{Column: "State", Values: []string{"__all__"}},
			}},
			`{"role":"north_mgr","policy":[{"column":"Region","values":["Northern","Eastern"]}]}`,
		},
	}

	for name, test := range policies {
		t.Run(name, func(t *testing.T) {
			db := getInitializedDbHandle(t)
			if err := LoadDbWithPolicies(db, &PolicySet{Policies: []Policy{test.input}}); err != nil {
				t.Fatalf("Error loading db with policies: %v\n", err)
			}
			fetch, err := GetPolicy(db, test.input.Role)
			if err != nil {
				t.Fatalf("Error getting policy: %v\n", err)
			}
			if fetch.ToJson() != test.output {
				t.Errorf("Policy mismatch: got %s, want %s\n", fetch.ToJson(), test.output)
			}
			db.Close()
		})
	}
}

func TestGetPolicyWorks(t *testing.T) {
	db := getInitializedDbHandle(t)
	defer db.Close()
	policy_set, err := LoadRolePolicies("testdata/valid_policy_set.json")
	if err != nil {
		t.Fatalf("Error loading role policies: %v\n", err)
	}
	if err := LoadDbWithPolicies(db, policy_set); err != nil {
		t.Fatalf("Error loading db with policies: %v\n", err)
	}

	policy, err := GetPolicy(db, "admin")
	if err != nil {
		t.Fatalf("Error getting policy: %v\n", err)
	}
	if policy.ToJson() != "null" {
		t.Errorf("Policy mismatch: got %s, want %s\n", policy.ToJson(), "null")
	}
}

func TestGetPolicyFailsIfRoleDoesNotExist(t *testing.T) {
	db := getInitializedDbHandle(t)
	defer db.Close()
	policy_set, err := LoadRolePolicies("testdata/valid_policy_set.json")
	if err != nil {
		t.Fatalf("Error loading role policies: %v\n", err)
	}
	if err := LoadDbWithPolicies(db, policy_set); err != nil {
		t.Fatalf("Error loading db with policies: %v\n", err)
	}

	_, err = GetPolicy(db, "does_not_exist")
	if err == nil {
		t.Error("Expected error getting policy for role that does not exist, but got none")
	}
}

func TestDbAlreadyInitializedWorks(t *testing.T) {
	t.Run("Uninitialized db not initialized", func(t *testing.T) {
		db := getDbHandle(t)
		defer db.Close()
		is_initialized := DbAlreadyInitialized(db)
		if is_initialized {
			t.Error("Db falsely reported as initialized")
		}
	})
	t.Run("Initialized db initialized", func(t *testing.T) {
		db := getInitializedDbHandle(t)
		defer db.Close()
		is_initialized := DbAlreadyInitialized(db)
		if !is_initialized {
			t.Error("Db falsely reported as uninitialized")
		}
	})

	partials := map[string]string{
		"policies": "create table if not exists policies(role varchar, control_column varchar, value varchar)",
		"roles":    "create table if not exists roles(role varchar unique)",
	}
	for tbl, exec_statment := range partials {
		t.Run(fmt.Sprintf("Uninitialized if only table is %s", tbl), func(t *testing.T) {
			db := getDbHandle(t)
			defer db.Close()
			if _, err := db.Exec(exec_statment); err != nil {
				t.Fatalf("Failed to create temporary table %v\n", err)
			}
			is_initialized := DbAlreadyInitialized(db)
			if is_initialized {
				t.Errorf("Db reported as initialized but only has %s\n", tbl)
			}
		})
	}
}

func TestOverwritePolicyWorks(t *testing.T) {
	// Setup: Two overlapping policy sets
	overlapping_policy_sets := []PolicySet{
		{Policies: []Policy{{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two"}}}}}},
		{Policies: []Policy{{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"three", "four"}}}}}},
	}
	db := getInitializedDbHandle(t)
	defer db.Close()
	for _, policy_set := range overlapping_policy_sets {
		if err := LoadDbWithPolicies(db, &policy_set); err != nil {
			t.Fatalf("Error loading db with policies: %v\n", err)
		}
	}

	// Test: Confirm only last policy set is in database
	policy, err := GetPolicy(db, "admin")
	if err != nil {
		t.Fatalf("Error getting policy: %v\n", err)
	}
	expected_policy := `{"role":"admin","policy":[{"column":"Region","values":["three","four"]}]}`
	if policy.ToJson() != expected_policy {
		t.Errorf("Policy mismatch: got %s, want %s\n", policy.ToJson(), expected_policy)
	}
}

func getInitializedDbHandle(t *testing.T) *sql.DB {
	t.Helper()
	db := getDbHandle(t)
	if err := InitDb(db); err != nil {
		db.Close()
		t.Fatalf("Error initializing db: %v\n", err)
	}
	return db
}

func getDbHandle(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Error opening db: %v\n", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Error pinging db: %v\n", err)
	}

	return db
}

func fetchOneRow(t *testing.T, db *sql.DB, query string, dest ...any) {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("Error querying db: %v\n", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("No rows found")
	}

	rows.Scan(dest...)
	if rows.Next() {
		t.Fatalf("Found too many rows (expected 1, got >1)")
	}
}
