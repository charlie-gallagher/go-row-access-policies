package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"

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

func main() {
	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}

	if err = InitDb(db); err != nil {
		log.Fatal(err)
	}

	// Load data from multiple config files
	err = LoadDbFromFile(db, "config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = LoadDbFromFile(db, "config_2.json")
	if err != nil {
		log.Fatal(err)
	}

	// Let's test it out
	ex_policy_item, err := GetPolicyItem(db, "north_eastern_sales_manager", "Region")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy_item.ToJson())
	ex_policy_item, err = GetPolicyItem(db, "north_eastern_sales_manager", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy_item.ToJson())
	ex_policy_item, err = GetPolicyItem(db, "admin", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy_item.ToJson())
	ex_policy_item, err = GetPolicyItem(db, "sales_manager", "Region")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy_item.ToJson())
	ex_policy_item, err = GetPolicyItem(db, "northwestern_sales_manager", "Region")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy_item.ToJson())

	ex_policy, err := GetPolicy(db, "admin")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
	ex_policy, err = GetPolicy(db, "northwestern_sales_manager")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
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
		`); err != nil {
		return err
	}
	return nil
}

// Load the database with policies from the config
func LoadDbWithPolicies(db *sql.DB, policy_set *PolicySet) error {
	for _, role_policy := range policy_set.Policies {
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
func GetPolicy(db *sql.DB, role string) (Policy, error) {
	rows, err := db.Query("select distinct control_column from policies where role = ?", role)
	if err != nil {
		return Policy{}, err
	}
	defer rows.Close()
	var control_columns []string
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			return Policy{}, err
		}
		control_columns = append(control_columns, column)
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
