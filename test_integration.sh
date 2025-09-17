#!/bin/bash
set -euo pipefail

# Integration test script for gosh
# Tests real SSH connections using hosts from .env file

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0

# Load environment variables from .env file
load_env() {
    if [[ ! -f .env ]]; then
        echo -e "${RED}ERROR: .env file not found${NC}"
        echo "Copy .env.example to .env and configure your test hosts"
        exit 1
    fi

    # Source the .env file
    set -a
    source .env
    set +a

    # Check required variables
    if [[ -z "${TEST_HOSTS:-}" ]]; then
        echo -e "${RED}ERROR: TEST_HOSTS not set in .env file${NC}"
        exit 1
    fi

    # Set defaults
    TEST_SSH_USER="${TEST_SSH_USER:-}"
    TEST_SSH_TIMEOUT="${TEST_SSH_TIMEOUT:-5}"
}

# Print test header
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  gosh Integration Test Suite${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo
}

# Print test status
print_test() {
    local test_name="$1"
    echo -e "${YELLOW}[TEST]${NC} $test_name"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
}

# Mark test as passed
test_passed() {
    echo -e "${GREEN}[PASS]${NC} Test passed"
    echo
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

# Mark test as failed
test_failed() {
    local reason="$1"
    echo -e "${RED}[FAIL]${NC} $reason"
    echo
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

# Run gosh command and capture output
run_gosh() {
    local cmd="$1"
    local hosts="$2"
    local user_flag=""

    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    # Use timeout to prevent hanging tests
    timeout "$TEST_SSH_TIMEOUT" ./build/gosh $user_flag -c "$cmd" $hosts 2>&1 || true
}

# Test basic functionality
test_basic_commands() {
    print_test "Basic Commands - date, uptime, hostname"

    # Test date command
    local output
    output=$(run_gosh "date" "$TEST_HOSTS")
    if [[ $output =~ [0-9]{4}-[0-9]{2}-[0-9]{2} ]] || [[ $output =~ [A-Z][a-z]{2}.*[0-9]{4} ]]; then
        test_passed
    else
        test_failed "Date command didn't return expected date format"
        return
    fi

    print_test "Uptime command"
    output=$(run_gosh "uptime" "$TEST_HOSTS")
    if [[ $output =~ "up" ]] || [[ $output =~ "load average" ]]; then
        test_passed
    else
        test_failed "Uptime command didn't return expected output"
        return
    fi

    print_test "Hostname command"
    output=$(run_gosh "hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Hostname command failed"
    fi
}

# Test file system commands (read-only)
test_filesystem_commands() {
    print_test "File system commands - ls, pwd, cat"

    # Test ls command
    local output
    output=$(run_gosh "ls /" "$TEST_HOSTS")
    if [[ $output =~ "bin" ]] || [[ $output =~ "usr" ]] || [[ $output =~ "etc" ]]; then
        test_passed
    else
        test_failed "ls / didn't show expected directories"
        return
    fi

    print_test "pwd command"
    output=$(run_gosh "pwd" "$TEST_HOSTS")
    if [[ $output =~ "/" ]]; then
        test_passed
    else
        test_failed "pwd didn't return a path"
        return
    fi

    print_test "cat /etc/hostname"
    output=$(run_gosh "cat /etc/hostname 2>/dev/null || hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Unable to read hostname"
    fi
}

# Test system information commands
test_system_info() {
    print_test "System information - uname, whoami, id"

    local output

    print_test "whoami command"
    output=$(run_gosh "whoami" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "whoami command failed"
        return
    fi

    print_test "id command"
    output=$(run_gosh "id" "$TEST_HOSTS")
    if [[ $output =~ "uid=" ]] || [[ $output =~ "gid=" ]]; then
        test_passed
    else
        test_failed "id command didn't return expected output"
    fi
}

# Test error handling
test_error_handling() {
    print_test "Error handling - invalid command"

    local output
    output=$(run_gosh "this_command_does_not_exist_12345" "$TEST_HOSTS")
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "not found" ]] || [[ $output =~ "command not found" ]]; then
        test_passed
    else
        test_failed "Invalid command should produce error"
    fi
}

# Test multiple hosts (if more than one host is configured)
test_multiple_hosts() {
    local host_count
    host_count=$(echo "$TEST_HOSTS" | wc -w)

    if [[ $host_count -gt 1 ]]; then
        print_test "Multiple hosts - parallel execution"

        local output
        output=$(run_gosh "echo 'test from \$(hostname)'" "$TEST_HOSTS")
        local output_lines
        output_lines=$(echo "$output" | grep -c "test from" || true)

        if [[ $output_lines -ge 1 ]]; then
            test_passed
        else
            test_failed "Multiple hosts test didn't return expected output from hosts"
        fi
    else
        echo -e "${YELLOW}[SKIP]${NC} Multiple hosts test (only one host configured)"
        echo
    fi
}

# Test color output
test_color_output() {
    print_test "Color output - default behavior"

    local output
    output=$(run_gosh "echo test" "$TEST_HOSTS")
    if [[ $output =~ \[31m ]] || [[ $output =~ \[32m ]] || [[ $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes not found in output"
        return
    fi

    print_test "No color output - --no-color flag"
    local user_flag=""
    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    output=$(timeout "$TEST_SSH_TIMEOUT" ./build/gosh $user_flag --no-color -c "echo test" $TEST_HOSTS 2>&1 || true)
    if [[ ! $output =~ \[31m ]] && [[ ! $output =~ \[32m ]] && [[ ! $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes found in --no-color output"
    fi
}

# Print test summary
print_summary() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  Test Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "Total tests:  $TESTS_TOTAL"
    echo -e "${GREEN}Passed tests: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed tests: $TESTS_FAILED${NC}"
    echo

    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}🎉 All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}❌ Some tests failed${NC}"
        exit 1
    fi
}

# Main test execution
main() {
    print_header

    # Load environment
    load_env

    echo -e "Test hosts: ${GREEN}$TEST_HOSTS${NC}"
    echo -e "SSH user: ${GREEN}${TEST_SSH_USER:-<default>}${NC}"
    echo -e "SSH timeout: ${GREEN}${TEST_SSH_TIMEOUT}s${NC}"
    echo

    # Check if gosh binary exists
    if [[ ! -f "./build/gosh" ]]; then
        echo -e "${RED}ERROR: ./build/gosh not found${NC}"
        echo "Run 'make build' first"
        exit 1
    fi

    # Run test suites
    test_basic_commands
    test_filesystem_commands
    test_system_info
    test_error_handling
    test_multiple_hosts
    test_color_output

    # Print results
    print_summary
}

# Run main function
main "$@"
