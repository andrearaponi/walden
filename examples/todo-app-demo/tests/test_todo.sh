#!/bin/sh
# Usage: tests/test_todo.sh [test-name]
# Runs one test by name, or every test. Each test runs in an empty temporary
# directory and proves one acceptance criterion; a test that passes prints
# "PASS: <name>".
set -e

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TODO="$PROJECT_DIR/src/todo.sh"

fail() {
    echo "FAIL: $1"
    exit 1
}

# R1.AC1: add appends the item to the todo file and confirms it
test_add() {
    sh "$TODO" add "Buy groceries" > /dev/null
    output=$(sh "$TODO" add "Write tests")
    [ "$output" = "Added: Write tests" ] || fail "add printed: $output"
    [ "$(sed -n 2p todos.txt)" = "TODO|Write tests" ] || fail "second line of the file: $(sed -n 2p todos.txt)"
}

# R1.AC2: add creates the todo file when it does not exist
test_add_creates_file() {
    [ ! -e todos.txt ] || fail "todos.txt exists before the first add"
    sh "$TODO" add "First item" > /dev/null
    [ "$(cat todos.txt)" = "TODO|First item" ] || fail "the file holds: $(cat todos.txt)"
}

# R2.AC1: list shows every item with its status and index
test_list() {
    sh "$TODO" add "Task one" > /dev/null
    sh "$TODO" add "Task two" > /dev/null
    expected="1. [TODO] Task one
2. [TODO] Task two"
    [ "$(sh "$TODO" list)" = "$expected" ] || fail "list printed: $(sh "$TODO" list)"
}

# R2.AC2: list says so when there is no item
test_list_empty() {
    [ "$(sh "$TODO" list)" = "No items." ] || fail "list printed: $(sh "$TODO" list)"
}

# R3.AC1: complete marks the item at that index as done
test_complete() {
    sh "$TODO" add "Task one" > /dev/null
    sh "$TODO" add "Task two" > /dev/null
    sh "$TODO" complete 2 > /dev/null
    expected="1. [TODO] Task one
2. [DONE] Task two"
    [ "$(sh "$TODO" list)" = "$expected" ] || fail "list printed: $(sh "$TODO" list)"
}

# R3.AC2: complete reports an index that is out of range
test_complete_out_of_range() {
    sh "$TODO" add "Only item" > /dev/null
    status=0
    output=$(sh "$TODO" complete 5 2>&1) || status=$?
    [ "$status" -eq 1 ] || fail "exit status $status"
    [ "$output" = "Error: index out of range" ] || fail "complete printed: $output"
}

run() {
    dir=$(mktemp -d)
    status=0
    ( cd "$dir" && "test_$1" ) || status=$?
    rm -rf "$dir"
    [ "$status" -eq 0 ] || exit "$status"
    echo "PASS: $1"
}

ALL="add add_creates_file list list_empty complete complete_out_of_range"

if [ $# -eq 0 ]; then
    for name in $ALL; do run "$name"; done
    exit 0
fi

case " $ALL " in
    *" $1 "*) run "$1" ;;
    *) echo "Unknown test: $1"; echo "Tests: $ALL"; exit 2 ;;
esac
