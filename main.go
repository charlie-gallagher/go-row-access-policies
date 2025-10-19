package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

type Config struct {
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

var db *sql.DB

func main() {
	// Read in the policies
	roles, err := loadRolePolicies("config.json")
	if err != nil {
		log.Fatal(err)
	}

	// Connect to database
	db, err = sql.Open("sqlite", "test-row-access.db")
	if err != nil {
		log.Fatal(err)
	}
	// Use Ping to actually test that it was successful
	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}

	// I'm not yet sure if deferring Close is best practice
	defer db.Close()

	if err = InitDb(); err != nil {
		log.Fatal(err)
	}

	// Load database with policies
	if err = LoadDbWithPolicies(roles); err != nil {
		log.Fatal(err)
	}

	// Look up some policies
	ex_policy, err := GetPolicy("north_eastern_sales_manager", "Region")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
	ex_policy, err = GetPolicy("north_eastern_sales_manager", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
	ex_policy, err = GetPolicy("admin", "State")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ex_policy.ToJson())
}

func loadRolePolicies(fname string) (*Config, error) {
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func InitDb() error {
	if _, err := db.Exec(`
	create table if not exists policies(role varchar, control_column varchar, value varchar);
	delete from policies;
		`); err != nil {
		return err
	}
	return nil
}

// Load the database with policies from the config
func LoadDbWithPolicies(roles *Config) error {
	for _, role_policy := range roles.Policies {
		for _, policy_item := range role_policy.Policy {
			for i, value := range policy_item.Values {
				// Skip the __all__ value if it's the first value
				if i == 0 && value == "__all__" {
					continue
				}
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
func GetPolicy(role, column string) (PolicyItem, error) {
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
