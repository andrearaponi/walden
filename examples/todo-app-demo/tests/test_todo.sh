#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TODO="$PROJECT_DIR/src/todo.sh"
PASS=0
FAIL=0

setup() {
    TEST_DIR=$(mktemp -d)
    cd "$TEST_DIR"
}

teardown() {
    cd "$PROJECT_DIR"
    rm -rf "$TEST_DIR"
}

assert_contains() {
    if echo "$1" | grep -q "$2"; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: expected output to contain '$2', got: $1"
        FAIL=$((FAIL + 1))
    fi
}

assert_exit_code() {
    if [ "$1" -eq "$2" ]; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: expected exit code $2, got $1"
        FAIL=$((FAIL + 1))
    fi
}

# Test: add creates item
test_add() {
    setup
    output=$(sh "$TODO" add "Buy groceries")
    assert_contains "$output" "Added"
    assert_contains "$(cat todos.txt)" "TODO|Buy groceries"
    teardown
}

# Test: list shows items
test_list() {
    setup
    sh "$TODO" add "Task one" > /dev/null
    sh "$TODO" add "Task two" > /dev/null
    output=$(sh "$TODO" list)
    assert_contains "$output" "1."
    assert_contains "$output" "Task one"
    assert_contains "$output" "2."
    teardown
}

# Test: list empty
test_list_empty() {
    setup
    output=$(sh "$TODO" list)
    assert_contains "$output" "No items"
    teardown
}

# Test: complete marks item done
test_complete() {
    setup
    sh "$TODO" add "Finish demo" > /dev/null
    sh "$TODO" complete 1 > /dev/null
    output=$(sh "$TODO" list)
    assert_contains "$output" "DONE"
    teardown
}

# Test: complete out of range
test_complete_out_of_range() {
    setup
    sh "$TODO" add "Only item" > /dev/null
    exit_code=0
    sh "$TODO" complete 5 > /dev/null 2>&1 || exit_code=$?
    assert_exit_code "$exit_code" 1
    teardown
}

# Run all tests
test_add
test_list
test_list_empty
test_complete
test_complete_out_of_range

echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
