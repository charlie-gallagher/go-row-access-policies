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

// Return a JSON string representation of the policy item
//
// At the moment, we only ever return individual policy items, so this is the
// only method we need.
func (pi *PolicyItem) ToJson() string {
	json, err := json.Marshal(pi)
	if err != nil {
		return "(error marshalling policy item)"
	}
	return string(json)
}

func main() {
	roles, err := LoadRolePolicies("config.json")
	if err != nil {
		log.Fatal(err)
	}

	var db *sql.DB
	db, err = sql.Open("sqlite", "test-row-access.db")
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

	if err = LoadDbWithPolicies(db, roles); err != nil {
		log.Fatal(err)
	}

	// Let's test it out
	ex_policy, err := GetPolicy(db, "north_eastern_sales_manager", "Region")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
	ex_policy, err = GetPolicy(db, "north_eastern_sales_manager", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
	ex_policy, err = GetPolicy(db, "admin", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
}

// Load the role policies from the config file
func LoadRolePolicies(fname string) (*PolicySet, error) {
	if err := ValidateConfig(fname); err != nil {
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
func ValidateConfig(fname string) error {
	schema_fname := "config_schema.json"
	c := jsonschema.NewCompiler()
	schema, err := c.Compile(schema_fname)
	if err != nil {
		return err
	}

	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return err
	}
	f.Close()
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

// Return a PolicyItem for this role and control column
func GetPolicy(db *sql.DB, role, column string) (PolicyItem, error) {
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
	return PolicyItem{Column: column, Values: column_values}, nil
}
