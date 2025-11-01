#!/usr/bin/env zsh


BASE_COMMAND=(./row_access --db ex.db)
RETURN_VALUE=0

init() {
    clean_all
    go build -o row_access .
}


clean_db() {
    if [[ -f ex.db ]]; then
        rm ex.db
    fi
}

clean_bin() {
    if [[ -f row_access ]]; then
        rm row_access
    fi
}

clean_all() {
    clean_db
    clean_bin
}

update_return_value() {
    if [[ $1 == "Failed:"* ]]; then
        RETURN_VALUE=1
    fi
    print "$1"
}


load_db() {
    clean_db
    local -a load_command=("${BASE_COMMAND[@]}")
    load_command+=(--load config.json)
    $load_command
    if (( $? != 0 )); then
        print "Failed: load command, command returned error code"
        return 1
    elif [[ ! -f ex.db ]]; then
        print "Failed: load command, ex.db not found"
        return 1
    fi
    return 0
}

load_db_2() {
    clean_db
    local -a load_command_1=("${BASE_COMMAND[@]}")
    local -a load_command_2=("${BASE_COMMAND[@]}")
    load_command_1+=(--load config.json)
    $load_command_1
    load_command_2+=(--load config_2.json)
    $load_command_2
    if (( $? != 0 )); then
        print "Failed: load command, command returned error code"
    elif [[ ! -f ex.db ]]; then
        print "Failed: load command, ex.db not found"
    fi
}

test_db_load() {
    clean_db
    local -a load_command=("${BASE_COMMAND[@]}")
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
    response="$(load_db)"
    if (( $? != 0 )); then
        print "$response"
        return 1
    fi
    local -a get_command=("${BASE_COMMAND[@]}")
    get_command+=(--get eastern_region_sales_manager)
    results=$( $get_command )
    if (( $? != 0 )); then
        print "Failed: get policy"
    else
        print "Successfully got policy: $results"
    fi
}

test_db_failed_fetch() {
    response="$(load_db)"
    if (( $? != 0 )); then
        print "$response"
        return 1
    fi
    local -a get_command=("${BASE_COMMAND[@]}")
    get_command+=(--get does_not_exist)
    results=$( $get_command )
    if (( $? == 0 )); then
        print "Failed: invalid role did not return error code"
    else
        print "Successfully errored on bad policy: $results"
    fi
}

test_can_load_multiple_configs() {
    response="$(load_db)"
    if (( $? != 0 )); then
        print "$response"
        return 1
    fi
    local found_roles=( $(sqlite3 ex.db 'select * from roles;' ) )
    if (( "${#found_roles}" < 4 )); then
        print "Failed: not enough roles in db"
    else
        print "Successfully loaded multiple policies"
    fi
}


test_role_name_validation() {
    local -a validate_command=("${BASE_COMMAND[@]}")
    tmp_file=$(mktemp)
    echo '{"policies":[{"role":"-admin-", "policy":[{"column":"Region", "values":["one", "two"]}]}]}' > $tmp_file
    validate_command+=(--load $tmp_file)
    $validate_command
    if (( $? != 0 )); then
        print "Successfully errored on invalid role name"
    else
        print "Failed: did not error on invalid role name"
    fi
}

test_cli_errors_for_load_and_get() {
    local -a load_command=("${BASE_COMMAND[@]}")
    load_command+=(--load config.json --get admin)
    $load_command
    if (( $? != 0 )); then
        print "Successfully errored for load and get"
    else
        print "Failed: did not error for load and get"
    fi
}

test_cli_errors_for_no_flags() {
    local -a no_flags_command=("${BASE_COMMAND[@]}")
    $no_flags_command
    if (( $? != 0 )); then
        print "Successfully errored for no flags"
    else
        print "Failed: did not error for no flags"
    fi
}

init
update_return_value "$(test_db_load)"
update_return_value "$(test_db_fetch)"
update_return_value "$(test_db_failed_fetch)"
update_return_value "$(test_can_load_multiple_configs)"
update_return_value "$(test_role_name_validation)"
update_return_value "$(test_cli_errors_for_load_and_get)"
update_return_value "$(test_cli_errors_for_no_flags)"
clean_all
exit $RETURN_VALUE