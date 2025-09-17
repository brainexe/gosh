#!/bin/bash
set -euo pipefail


make build

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

# Run gosh command in direct mode and capture output
run_gosh_direct() {
    local cmd="$1"
    local hosts="$2"
    local user_flag=""

    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    # Use timeout to prevent hanging tests
    timeout "$TEST_SSH_TIMEOUT" ./build/gosh $user_flag -c "$cmd" $hosts 2>&1 || true
}

# Run gosh command in interactive mode and capture output
run_gosh_interactive() {
    local cmd="$1"
    local hosts="$2"
    local user_flag=""

    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    # Use timeout to prevent hanging tests, simulate interactive input
    timeout "$TEST_SSH_TIMEOUT" bash -c "
        ./build/gosh $user_flag $hosts << 'EOF'
$cmd
exit
EOF
" 2>&1 || true
}

# Run gosh command and capture output (backwards compatibility)
run_gosh() {
    run_gosh_direct "$1" "$2"
}

# Test basic functionality in both direct and interactive modes
test_basic_commands() {
    # Test date command - Direct mode
    print_test "Basic Commands - date (direct mode)"
    local output
    output=$(run_gosh_direct "date" "$TEST_HOSTS")
    echo -e "${YELLOW}DEBUG: Actual output:${NC}"
    echo "$output"
    echo
    if [[ $output =~ [0-9]{4}-[0-9]{2}-[0-9]{2} ]] || [[ $output =~ [A-Z][a-z]{2}.*[0-9]{4} ]]; then
        test_passed
    else
        test_failed "Date command in direct mode didn't return expected date format"
        return
    fi

    # Test date command - Interactive mode
    print_test "Basic Commands - date (interactive mode)"
    output=$(run_gosh_interactive "date" "$TEST_HOSTS")
    if [[ $output =~ [0-9]{4}-[0-9]{2}-[0-9]{2} ]] || [[ $output =~ [A-Z][a-z]{2}.*[0-9]{4} ]]; then
        test_passed
    else
        test_failed "Date command in interactive mode didn't return expected date format"
        return
    fi

    # Test uptime command - Direct mode
    print_test "Uptime command (direct mode)"
    output=$(run_gosh_direct "uptime" "$TEST_HOSTS")
    if [[ $output =~ "up" ]] || [[ $output =~ "load average" ]]; then
        test_passed
    else
        test_failed "Uptime command in direct mode didn't return expected output"
        return
    fi

    # Test uptime command - Interactive mode
    print_test "Uptime command (interactive mode)"
    output=$(run_gosh_interactive "uptime" "$TEST_HOSTS")
    if [[ $output =~ "up" ]] || [[ $output =~ "load average" ]]; then
        test_passed
    else
        test_failed "Uptime command in interactive mode didn't return expected output"
        return
    fi

    # Test hostname command - Direct mode
    print_test "Hostname command (direct mode)"
    output=$(run_gosh_direct "hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Hostname command in direct mode failed"
        return
    fi

    # Test hostname command - Interactive mode
    print_test "Hostname command (interactive mode)"
    output=$(run_gosh_interactive "hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Hostname command in interactive mode failed"
    fi
}

# Test file system commands (read-only) in both direct and interactive modes
test_filesystem_commands() {
    # Test ls command - Direct mode
    print_test "File system commands - ls / (direct mode)"
    local output
    output=$(run_gosh_direct "ls /" "$TEST_HOSTS")
    echo -e "${YELLOW}DEBUG: Actual output:${NC}"
    echo "$output"
    echo
    if [[ $output =~ "bin" ]] || [[ $output =~ "usr" ]] || [[ $output =~ "etc" ]]; then
        test_passed
    else
        test_failed "ls / in direct mode didn't show expected directories"
        return
    fi

    # Test ls command - Interactive mode
    print_test "File system commands - ls / (interactive mode)"
    output=$(run_gosh_interactive "ls /" "$TEST_HOSTS")
    if [[ $output =~ "bin" ]] || [[ $output =~ "usr" ]] || [[ $output =~ "etc" ]]; then
        test_passed
    else
        test_failed "ls / in interactive mode didn't show expected directories"
        return
    fi

    # Test pwd command - Direct mode
    print_test "pwd command (direct mode)"
    output=$(run_gosh_direct "pwd" "$TEST_HOSTS")
    if [[ $output =~ "/" ]]; then
        test_passed
    else
        test_failed "pwd in direct mode didn't return a path"
        return
    fi

    # Test pwd command - Interactive mode
    print_test "pwd command (interactive mode)"
    output=$(run_gosh_interactive "pwd" "$TEST_HOSTS")
    if [[ $output =~ "/" ]]; then
        test_passed
    else
        test_failed "pwd in interactive mode didn't return a path"
        return
    fi

    # Test cat /etc/hostname - Direct mode
    print_test "cat /etc/hostname (direct mode)"
    output=$(run_gosh_direct "cat /etc/hostname 2>/dev/null || hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Unable to read hostname in direct mode"
        return
    fi

    # Test cat /etc/hostname - Interactive mode
    print_test "cat /etc/hostname (interactive mode)"
    output=$(run_gosh_interactive "cat /etc/hostname 2>/dev/null || hostname" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "Unable to read hostname in interactive mode"
    fi
}

# Test system information commands in both direct and interactive modes
test_system_info() {
    local output

    # Test whoami command - Direct mode
    print_test "whoami command (direct mode)"
    output=$(run_gosh_direct "whoami" "$TEST_HOSTS")
    echo -e "${YELLOW}DEBUG: Actual output:${NC}"
    echo "$output"
    echo
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "whoami command in direct mode failed"
        return
    fi

    # Test whoami command - Interactive mode
    print_test "whoami command (interactive mode)"
    output=$(run_gosh_interactive "whoami" "$TEST_HOSTS")
    if [[ -n "$output" ]] && [[ ! $output =~ "ERROR" ]]; then
        test_passed
    else
        test_failed "whoami command in interactive mode failed"
        return
    fi

    # Test id command - Direct mode
    print_test "id command (direct mode)"
    output=$(run_gosh_direct "id" "$TEST_HOSTS")
    if [[ $output =~ "uid=" ]] || [[ $output =~ "gid=" ]]; then
        test_passed
    else
        test_failed "id command in direct mode didn't return expected output"
        return
    fi

    # Test id command - Interactive mode
    print_test "id command (interactive mode)"
    output=$(run_gosh_interactive "id" "$TEST_HOSTS")
    if [[ $output =~ "uid=" ]] || [[ $output =~ "gid=" ]]; then
        test_passed
    else
        test_failed "id command in interactive mode didn't return expected output"
    fi
}

# Test error handling in both direct and interactive modes
test_error_handling() {
    # Test invalid command - Direct mode
    print_test "Error handling - invalid command (direct mode)"
    local output
    output=$(run_gosh_direct "this_command_does_not_exist_12345" "$TEST_HOSTS")
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "not found" ]] || [[ $output =~ "command not found" ]]; then
        test_passed
    else
        test_failed "Invalid command in direct mode should produce error"
        return
    fi

    # Test invalid command - Interactive mode
    print_test "Error handling - invalid command (interactive mode)"
    output=$(run_gosh_interactive "this_command_does_not_exist_12345" "$TEST_HOSTS")
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "not found" ]] || [[ $output =~ "command not found" ]]; then
        test_passed
    else
        test_failed "Invalid command in interactive mode should produce error"
    fi
}

# Test multiple hosts (if more than one host is configured) in both direct and interactive modes
test_multiple_hosts() {
    local host_count
    host_count=$(echo "$TEST_HOSTS" | wc -w)

    if [[ $host_count -gt 1 ]]; then
        # Test multiple hosts - Direct mode
        print_test "Multiple hosts - parallel execution (direct mode)"
        local output
        output=$(run_gosh_direct "echo 'test from \$(hostname)'" "$TEST_HOSTS")
        local output_lines
        output_lines=$(echo "$output" | grep -c "test from" || true)

        if [[ $output_lines -ge 1 ]]; then
            test_passed
        else
            test_failed "Multiple hosts test in direct mode didn't return expected output from hosts"
            return
        fi

        # Test multiple hosts - Interactive mode
        print_test "Multiple hosts - parallel execution (interactive mode)"
        output=$(run_gosh_interactive "echo 'test from \$(hostname)'" "$TEST_HOSTS")
        output_lines=$(echo "$output" | grep -c "test from" || true)

        if [[ $output_lines -ge 1 ]]; then
            test_passed
        else
            test_failed "Multiple hosts test in interactive mode didn't return expected output from hosts"
        fi
    else
        echo -e "${YELLOW}[SKIP]${NC} Multiple hosts test (only one host configured)"
        echo
    fi
}

# Test color output in both direct and interactive modes
test_color_output() {
    # Test color output - Direct mode
    print_test "Color output - default behavior (direct mode)"
    local output
    output=$(run_gosh_direct "echo test" "$TEST_HOSTS")
    if [[ $output =~ \[31m ]] || [[ $output =~ \[32m ]] || [[ $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes not found in direct mode output"
        return
    fi

    # Test color output - Interactive mode
    print_test "Color output - default behavior (interactive mode)"
    output=$(run_gosh_interactive "echo test" "$TEST_HOSTS")
    if [[ $output =~ \[31m ]] || [[ $output =~ \[32m ]] || [[ $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes not found in interactive mode output"
        return
    fi

    # Test no color output - Direct mode with --no-color flag
    print_test "No color output - --no-color flag (direct mode)"
    local user_flag=""
    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    output=$(timeout "$TEST_SSH_TIMEOUT" ./build/gosh $user_flag --no-color -c "echo test" $TEST_HOSTS 2>&1 || true)
    if [[ ! $output =~ \[31m ]] && [[ ! $output =~ \[32m ]] && [[ ! $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes found in --no-color direct mode output"
        return
    fi

    # Test no color output - Interactive mode with --no-color flag
    print_test "No color output - --no-color flag (interactive mode)"
    user_flag=""
    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi

    output=$(timeout "$TEST_SSH_TIMEOUT" bash -c "
        ./build/gosh $user_flag --no-color $TEST_HOSTS << 'EOF'
echo test
exit
EOF
" 2>&1 || true)
    if [[ ! $output =~ \[31m ]] && [[ ! $output =~ \[32m ]] && [[ ! $output =~ \[33m ]]; then
        test_passed
    else
        test_failed "Color codes found in --no-color interactive mode output"
    fi
}

# Test SSH connection failures and error handling
test_connection_failures() {
    # Test connection to non-existent host
    print_test "Connection failure - non-existent host (direct mode)"
    local output
    output=$(timeout 10 ./build/gosh -c "echo test" nonexistent.host.invalid.12345 2>&1 || true)
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "could not resolve hostname" ]] || [[ $output =~ "Connection refused" ]] || [[ $output =~ "Name or service not known" ]]; then
        test_passed
    else
        test_failed "Expected connection error for non-existent host"
        return
    fi

    # Test connection timeout with unreachable IP
    print_test "Connection timeout - unreachable IP (direct mode)"
    output=$(timeout 10 ./build/gosh -c "echo test" 192.0.2.1 2>&1 || true) # RFC5737 test IP
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "Connection timed out" ]] || [[ $output =~ "Network is unreachable" ]]; then
        test_passed
    else
        test_failed "Expected timeout error for unreachable IP"
        return
    fi

    # Test invalid SSH port
    print_test "Connection failure - invalid SSH port (direct mode)"
    output=$(timeout 10 ./build/gosh -c "echo test" localhost:99999 2>&1 || true)
    if [[ $output =~ "ERROR" ]] || [[ $output =~ "Connection refused" ]] || [[ $output =~ "port" ]]; then
        test_passed
    else
        test_failed "Expected connection error for invalid port"
    fi
}

# Test file upload functionality (requires real SSH connection)
test_file_upload_functionality() {
    # Create test file for upload
    local test_file="/tmp/gosh_upload_test.txt"
    echo "This is a test file for gosh upload functionality" > "$test_file"
    echo "Created at: $(date)" >> "$test_file"
    
    # Test file upload - Interactive mode
    print_test "File upload - interactive mode"
    local user_flag=""
    if [[ -n "$TEST_SSH_USER" ]]; then
        user_flag="-u $TEST_SSH_USER"
    fi
    
    local output
    output=$(timeout "$TEST_SSH_TIMEOUT" bash -c "
        ./build/gosh $user_flag $TEST_HOSTS << 'EOF'
:upload $test_file
exit
EOF
" 2>&1 || true)
    
    if [[ $output =~ "Upload successful" ]] || [[ $output =~ "uploaded" ]]; then
        test_passed
        
        # Verify file was uploaded by checking if it exists on remote host
        print_test "File upload verification - file exists on remote"
        local verify_output
        verify_output=$(run_gosh_direct "ls -la gosh_upload_test.txt && cat gosh_upload_test.txt" "$TEST_HOSTS")
        if [[ $verify_output =~ "This is a test file for gosh upload functionality" ]]; then
            test_passed
            
            # Clean up remote file
            run_gosh_direct "rm -f gosh_upload_test.txt" "$TEST_HOSTS" > /dev/null 2>&1 || true
        else
            test_failed "Uploaded file not found or content mismatch on remote host"
        fi
    else
        test_failed "File upload failed or no success message"
    fi
    
    # Test upload of non-existent file
    print_test "File upload error - non-existent file"
    output=$(timeout "$TEST_SSH_TIMEOUT" bash -c "
        ./build/gosh $user_flag $TEST_HOSTS << 'EOF'
:upload /nonexistent/file/path.txt
exit
EOF
" 2>&1 || true)
    
    if [[ $output =~ "does not exist" ]] || [[ $output =~ "Error:" ]]; then
        test_passed
    else
        test_failed "Expected error for non-existent file upload"
    fi
    
    # Clean up test file
    rm -f "$test_file"
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
    test_connection_failures
    test_file_upload_functionality

    # Print results
    print_summary
}

# Run main function
main "$@"
