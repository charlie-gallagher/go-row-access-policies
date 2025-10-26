package main

import (
	"database/sql"
	"fmt"
	"github.com/spf13/pflag"
	"os"
)

// I confess I wrote a weak version of this help text and then had Cursor
// make it better. It used man-page formatting instead of help-text formatting,
// but that's ok.
const help_text = `ROWCTRL(1)                    User Commands                   ROWCTRL(1)

NAME
       rowctrl - row access control policy management tool

SYNOPSIS
       rowctrl [OPTIONS] --db FILE --load CONFIG
       rowctrl [OPTIONS] --db FILE --get ROLE
       rowctrl [-h|--help]
       rowctrl [-v|--verbose]

DESCRIPTION
       rowctrl is a command-line tool for managing row-level access control
       policies in business intelligence applications. It provides functionality
       to load policy configurations into a SQLite database and retrieve
       policies for specific roles.

       The tool implements a separation of policy and enforcement, where this
       module controls the policy definitions and another module handles
       enforcement when necessary.

COMMANDS
       --load CONFIG
              Load policy configurations from a JSON configuration file into
              the specified database. The configuration file must conform to
              the JSON schema defined in config_schema.json.

       --get ROLE
              Retrieve and display the access policy for the specified role
              from the database.

OPTIONS
       -h, --help
              Display this help message and exit.

       -v, --verbose
              Enable verbose output mode for detailed operation information.

       --db FILE
              Specify the SQLite database file to use for policy storage and
              retrieval. This option is required for --load and --get commands.

CONFIGURATION FILE FORMAT
       The configuration file is a JSON document containing an array of policy
       definitions. Each policy consists of a role name and an array of policy
       items that define column access rules.

       Example configuration structure:
              {
                "policies": [
                  {
                    "role": "admin",
                    "policy": [
                      {"column": "Region", "values": ["__all__"]},
                      {"column": "State", "values": ["__all__"]}
                    ]
                  },
                  {
                    "role": "eastern_region_sales_manager",
                    "policy": [
                      {"column": "Region", "values": ["Eastern"]},
                      {"column": "State", "values": ["__all__"]}
                    ]
                  }
                ]
              }

       Special Values:
              "__all__"  Grants access to all values for the specified column

EXAMPLES
       Load policies from a configuration file:
              rowctrl --db policies.db --load config.json

       Retrieve policy for a specific role:
              rowctrl --db policies.db --get admin

       Enable verbose output:
              rowctrl --verbose --db policies.db --get eastern_region_sales_manager

       Save output to a file:
              rowctrl --db policies.db --get pa_sales_manager --output policy.json

FILES
       config_schema.json
              JSON schema file that defines the structure and validation rules
              for policy configuration files.

AUTHOR
       Charlie Gallagher, October 2025

SEE ALSO
       sqlite3(1), json(1)

BUGS
       Report bugs and feature requests to the project repository.

COPYRIGHT
       This is free software; see the source for copying conditions.`

func main() {
	var verbose bool
	var help bool
	var db_file string
	var config_file string
	var role string

	pflag.BoolVarP(&verbose, "verbose", "v", false, "enable verbose mode")
	pflag.BoolVarP(&help, "help", "h", false, "display help message")
	pflag.StringVarP(&db_file, "db", "d", "", "database file")
	pflag.StringVarP(&config_file, "load", "l", "", "config file to load into database")
	pflag.StringVarP(&role, "get", "g", "", "role to get policy for from database")

	pflag.Parse()

	if verbose {
		fmt.Println("Verbose mode enabled (but does nothing yet)")
	}

	if help {
		fmt.Println(help_text)
		os.Exit(0)
	}

	if config_file != "" && role != "" {
		fmt.Println("Error: --load and --get cannot be used together")
		os.Exit(1)
	}

	if config_file == "" && role == "" {
		fmt.Println("Error: either --help, --load or --get must be specified")
		os.Exit(1)
	}

	if db_file == "" {
		fmt.Println("Error: --db option is required")
		os.Exit(1)
	}

	db, err := getFileDbHandle(db_file)
	if err != nil {
		fmt.Println("Error getting db handle:", err)
		os.Exit(1)
	}
	defer db.Close()

	if config_file != "" {
		policy_set, err := LoadRolePolicies(config_file)
		if err != nil {
			fmt.Println("Error loading policies:", err)
			os.Exit(1)
		}
		if err = InitDb(db); err != nil {
			fmt.Println("Error initializing db:", err)
			os.Exit(1)
		}
		if err = LoadDbWithPolicies(db, policy_set); err != nil {
			fmt.Println("Error loading policies into db:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if role != "" {
		policy, err := GetPolicy(db, role)
		if err != nil {
			fmt.Printf("Error getting policy for role %s: %v\n", role, err)
			os.Exit(1)
		}
		fmt.Println(policy.ToJson())
		os.Exit(0)
	}
}

func getFileDbHandle(fname string) (*sql.DB, error) {
	var err error
	var db *sql.DB
	db, err = sql.Open("sqlite", fname)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
