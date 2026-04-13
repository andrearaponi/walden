#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TODO="$PROJECT_DIR/src/todo.sh"
TESTS="$PROJECT_DIR/tests/test_todo.sh"

case "$1" in
    1.1)
        # Verify add subcommand exists and works
        dir=$(mktemp -d)
        cd "$dir"
        output=$(sh "$TODO" add "verify item")
        echo "$output" | grep -q "Added"
        grep -q "TODO|verify item" todos.txt
        rm -rf "$dir"
        echo "PASS: 1.1 add subcommand"
        ;;
    1.2)
        # Verify list and complete subcommands work
        dir=$(mktemp -d)
        cd "$dir"
        sh "$TODO" add "test item" > /dev/null
        sh "$TODO" list | grep -q "1\."
        sh "$TODO" complete 1 > /dev/null
        sh "$TODO" list | grep -q "DONE"
        rm -rf "$dir"
        echo "PASS: 1.2 list and complete subcommands"
        ;;
    2.1)
        # Run the full test suite
        sh "$TESTS"
        ;;
    *)
        echo "Unknown task ID: $1"
        echo "Usage: verify.sh <task-id>"
        echo "Valid task IDs: 1.1, 1.2, 2.1"
        exit 1
        ;;
esac
