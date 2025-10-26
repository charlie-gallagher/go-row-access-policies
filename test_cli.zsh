#!/usr/bin/env zsh

clean() {
    if [[ -f ex.db ]]; then
        rm ex.db
    fi
}

local -a base_command=(go run . --db ex.db)

load_db() {
    clean
    local -a load_command=("${base_command[@]}")
    load_command+=(--load config.json)
    $load_command
    if (( $? != 0 )); then
        print "Failed: load command, command returned error code"
    elif [[ ! -f ex.db ]]; then
        print "Failed: load command, ex.db not found"
    fi
}

test_db_load() {
    clean
    local -a load_command=("${base_command[@]}")
    load_command+=(--load config.json)

    $load_command
    if (( $? != 0 )); then
        print "Failed: load command, command returned error code"
    elif [[ ! -f ex.db ]]; then
        print "Failed: load command, ex.db not found"
    else
        print "Successfully loaded database"
    fi
}

test_db_fetch() {
    load_db
    local -a get_command=("${base_command[@]}")
    get_command+=(--get eastern_region_sales_manager)
    results=$( $get_command )
    if (( $? != 0 )); then
        print "Failed: get policy"
    else
        print "Successfully got policy: $results"
    fi
}

test_db_failed_fetch() {
    load_db
    local -a get_command=("${base_command[@]}")
    get_command+=(--get does_not_exist)
    results=$( $get_command )
    if (( $? == 0 )); then
        print "Failed: invalid role did not return error code"
    else
        print "Successfully errored on bad policy: $results"
    fi
}

test_db_load
test_db_fetch
test_db_failed_fetch
clean