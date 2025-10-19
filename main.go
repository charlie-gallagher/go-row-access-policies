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

type PolicyItem struct {
	Column string   `json:"column"`
	Values []string `json:"values"`
}

type Policy struct {
	Role   string `json:"role"`
	Policy []PolicyItem `json:"policy"`
}

var db *sql.DB

func main() {
	// Read in the policies
	roles, err := readPolicies("config.json")
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

	// I'm not yet sure if defering Close is best practice
	defer db.Close()

	if err = InitDb(); err != nil {
		log.Fatal(err)
	}

	// Iterate over the policies and insert them into the database
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
					log.Fatal(err)
				}
			}
		}
	}
	fmt.Println("Policies inserted successfully")
}

func readPolicies(fname string) (*Config, error) {
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