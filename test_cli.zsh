#!/usr/bin/env zsh

clean() {
    if [[ -f ex.db ]]; then
        rm ex.db
    fi
}

RETURN_VALUE=0

# usage: update_return_value "test string"
#
# If the test string starts with "Failed:", then set return_value to 1
# otherwise, leave return_value alone.
update_return_value() {
    if [[ $1 == "Failed:"* ]]; then
        RETURN_VALUE=1
    fi
    print "$1"
}

local -a base_command=(go run . --db ex.db)

load_db() {
    clean
    local -a load_command=("${base_command[@]}")
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
    clean
    local -a load_command_1=("${base_command[@]}")
    local -a load_command_2=("${base_command[@]}")
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
    response="$(load_db)"
    if (( $? != 0 )); then
        print "$response"
        return 1
    fi
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
    response="$(load_db)"
    if (( $? != 0 )); then
        print "$response"
        return 1
    fi
    local -a get_command=("${base_command[@]}")
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
    local -a validate_command=("${base_command[@]}")
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
update_return_value "$(test_db_load)"
update_return_value "$(test_db_fetch)"
update_return_value "$(test_db_failed_fetch)"
update_return_value "$(test_can_load_multiple_configs)"
update_return_value "$(test_role_name_validation)"
clean
exit $RETURN_VALUE