package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"

	"github.com/santhosh-tekuri/jsonschema/v6"

	_ "modernc.org/sqlite"
)

const json_schema_fname = "config_schema.json"

var ErrNoSuchRole = errors.New("no such role")

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

func InitDb(db *SqliteDB) error {
	if err := db.Setup(); err != nil {
		return err
	}
	return nil
}

func DbAlreadyInitialized(db *SqliteDB) bool {
	tables, err := db.ListTables()
	if err != nil {
		return false
	}
	if len(tables) != 2 {
		return false
	}
	if !slices.Equal(tables, []string{"policies", "roles"}) {
		return false
	}
	return true
}

// Load the database with policies from the config
func LoadDbWithPolicies(db *SqliteDB, policy_set *PolicySet) error {
	insert_statement, err := db.Prepare("insert into policies (role, control_column, value) values (?, ?, ?)")
	if err != nil {
		return err
	}
	defer insert_statement.Close()
	for _, role_policy := range policy_set.Policies {
		// First, add role to `roles` table, if not already there
		was_created, err := tryAddRoleToRolesTable(db, role_policy.Role)
		if err != nil {
			return err
		}

		// If the role already exists, truncate all of its policies
		if !was_created {
			if err := db.Exec("delete from policies where role = ?", role_policy.Role); err != nil {
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
				if _, err := insert_statement.Exec(role_policy.Role, policy_item.Column, value); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Try to add role to roles table
//
// Returns bool indicating if role was added to table (false=existed beforehand)
func tryAddRoleToRolesTable(db *SqliteDB, role string) (bool, error) {
	// Validate role name
	if !IsValidRoleName(role) {
		return false, fmt.Errorf("invalid role name: %s", role)
	}
	// Check if role already exists
	rows, err := db.Select("select role from roles where role = ?", role)
	if err != nil {
		return false, err
	}
	if len(rows) == 1 {
		return false, nil
	}
	// Add role to table
	err = db.Exec("insert into roles (role) values (?)", role)
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

func LoadDbFromFile(db *SqliteDB, fname string) error {
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
func GetPolicy(db *SqliteDB, role string) (Policy, error) {
	// First, confirm the role exists
	rows, err := db.Select("select role from roles where role = ?", role)
	if err != nil {
		return Policy{}, err
	}
	if len(rows) != 1 {
		return Policy{}, fmt.Errorf("%w: role `%s` does not exist", ErrNoSuchRole, role)
	}

	// Now return the role data
	rows, err = db.Select("select distinct control_column from policies where role = ?", role)
	if err != nil {
		return Policy{}, err
	}
	control_columns := []string{}
	for _, row := range rows {
		// TODO: get rid of type assertions if possible
		control_columns = append(control_columns, row["control_column"].(string))
	}
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
func GetPolicyItem(db *SqliteDB, role, column string) (PolicyItem, error) {
	var column_values []string
	rows, err := db.Select("select value from policies where role = ? and control_column = ?", role, column)
	if err != nil {
		return PolicyItem{}, err
	}
	column_values = []string{}
	for _, val := range rows {
		column_values = append(column_values, val["value"].(string))
	}

	if len(column_values) == 0 {
		return PolicyItem{}, nil
	}
	return PolicyItem{Column: column, Values: column_values}, nil
}
