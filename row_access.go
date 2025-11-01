package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"regexp"

	_ "modernc.org/sqlite"
)

const json_schema_fname = "config_schema.json"

type PolicySet struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Role   string       `json:"role"`
	Policy []PolicyItem `json:"policy"`
}

type PolicyItem struct {
	Column string   `json:"column"`
	Values []string `json:"values"`
}

// Return a JSON string representation of the policy
func (p *Policy) ToJson() string {
	if len(p.Policy) == 0 {
		return "null"
	}
	json, err := json.Marshal(p)
	if err != nil {
		return "(error marshalling policy)"
	}
	return string(json)
}

// Return a JSON string representation of the policy item
func (pi *PolicyItem) ToJson() string {
	if len(pi.Values) == 0 {
		return "null"
	}
	json, err := json.Marshal(pi)
	if err != nil {
		return "(error marshalling policy item)"
	}
	return string(json)
}

// Load the role policies from the config file
func LoadRolePolicies(fname string) (*PolicySet, error) {
	if err := ValidateConfigFile(fname); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var policy_set PolicySet
	if err := json.Unmarshal(data, &policy_set); err != nil {
		return nil, err
	}
	return &policy_set, nil
}

// Validate the config file against the schema
func ValidateConfigFile(fname string) error {
	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}

	return ValidateConfig(data)
}

func ValidateConfig(data []byte) error {
	var inst any
	err := json.Unmarshal(data, &inst)
	if err != nil {
		return err
	}
	c := jsonschema.NewCompiler()
	schema, err := c.Compile(json_schema_fname)
	if err != nil {
		return err
	}

	if err := schema.Validate(inst); err != nil {
		return err
	}
	return nil
}

func InitDb(db *sql.DB) error {
	if _, err := db.Exec(`
	create table if not exists policies(role varchar, control_column varchar, value varchar);
	delete from policies;
	create table if not exists roles(role varchar unique);
	delete from roles;`); err != nil {
		return err
	}
	return nil
}

func DbAlreadyInitialized(db *sql.DB) bool {
	rows, err := db.Query("select count(*) from sqlite_master where type = 'table' and name in ('roles', 'policies')")
	if err != nil {
		return false
	}
	defer rows.Close()
	found_any_tables := rows.Next()
	if !found_any_tables {
		return false
	}
	var n int
	if err = rows.Scan(&n); err != nil {
		fmt.Printf("Error scanning db, %v\n", err)
		return false
	}
	return n == 2
}

// Load the database with policies from the config
func LoadDbWithPolicies(db *sql.DB, policy_set *PolicySet) error {
	for _, role_policy := range policy_set.Policies {
		// First, add role to `roles` table, if not already there
		was_created, err := tryAddRoleToRolesTable(db, role_policy.Role)
		if err != nil {
			return err
		}

		// If the role already exists, truncate all of its policies
		if !was_created {
			if _, err := db.Exec("delete from policies where role = ?", role_policy.Role); err != nil {
				return err
			}
		}
		for _, policy_item := range role_policy.Policy {
			// If the only policy item is __all__, then we don't need to insert any policies
			if len(policy_item.Values) == 1 && policy_item.Values[0] == "__all__" {
				continue
			}
			// Otherwise, insert the policies
			for _, value := range policy_item.Values {
				if _, err := db.Exec(`
					insert into policies (role, control_column, value) values (?, ?, ?);
					`, role_policy.Role, policy_item.Column, value); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func tryAddRoleToRolesTable(db *sql.DB, role string) (bool, error) {
	// Validate role name
	if !IsValidRoleName(role) {
		return false, fmt.Errorf("invalid role name: %s", role)
	}
	// Check if role already exists
	rows, err := db.Query("select role from roles where role = ?", role)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if rows.Next() {
		return false, nil
	}
	// Add role to table
	_, err = db.Exec("insert into roles (role) values (?)", role)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Return true if the role name is valid, false otherwise
//
// A valid role name must:
// - Start and end with a letter or number
// - Contain only letters, numbers, hyphens, and underscores
// - Be between 1 and 255 characters long
func IsValidRoleName(role string) bool {
	return regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]+[a-zA-Z0-9]$`).MatchString(role) && len(role) > 0 && len(role) <= 255
}

func LoadDbFromFile(db *sql.DB, fname string) error {
	policy_set, err := LoadRolePolicies(fname)
	if err != nil {
		return err
	}
	if err = LoadDbWithPolicies(db, policy_set); err != nil {
		return err
	}
	return nil
}

// For a given role, return all policy items
//
// This fetches the various control columns, then calls GetPolicyItem in a loop
// until that list is exhausted. This is not an efficient way to carry out the
// task, but it's easier to implement.
//
// Returns an error if the role does not exist.
func GetPolicy(db *sql.DB, role string) (Policy, error) {
	// First, confirm the role exists
	rows, err := db.Query("select role from roles where role = ?", role)
	if err != nil {
		return Policy{}, err
	}
	if !rows.Next() {
		rows.Close()
		return Policy{}, fmt.Errorf("role `%s` does not exist", role)
	}
	rows.Close()

	// Now return the role data
	rows, err = db.Query("select distinct control_column from policies where role = ?", role)
	if err != nil {
		return Policy{}, err
	}
	var control_columns []string
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			rows.Close()
			return Policy{}, err
		}
		control_columns = append(control_columns, column)
	}
	rows.Close()
	policy := Policy{Role: role}
	for _, cc := range control_columns {
		pi, err := GetPolicyItem(db, role, cc)
		if err != nil {
			return Policy{}, err
		}
		policy.Policy = append(policy.Policy, pi)
	}
	return policy, nil
}

// Return a PolicyItem for this role and control column
func GetPolicyItem(db *sql.DB, role, column string) (PolicyItem, error) {
	var column_values []string
	rows, err := db.Query("select value from policies where role = ? and control_column = ?", role, column)
	if err != nil {
		return PolicyItem{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return PolicyItem{}, err
		}
		column_values = append(column_values, v)
	}
	// If there are no values, return an empty policy item
	if len(column_values) == 0 {
		return PolicyItem{}, nil
	}
	return PolicyItem{Column: column, Values: column_values}, nil
}
