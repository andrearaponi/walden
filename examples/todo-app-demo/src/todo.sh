#!/bin/sh
set -e

DATA_FILE="todos.txt"

usage() {
    echo "Usage: todo.sh <add|list|complete> [args]"
    exit 1
}

cmd_add() {
    if [ -z "$1" ]; then
        echo "Error: description required"
        exit 1
    fi
    echo "TODO|$*" >> "$DATA_FILE"
    echo "Added: $*"
}

cmd_list() {
    if [ ! -f "$DATA_FILE" ] || [ ! -s "$DATA_FILE" ]; then
        echo "No items."
        return
    fi
    i=1
    while IFS='|' read -r status desc; do
        printf "%d. [%s] %s\n" "$i" "$status" "$desc"
        i=$((i + 1))
    done < "$DATA_FILE"
}

cmd_complete() {
    if [ -z "$1" ]; then
        echo "Error: index required"
        exit 1
    fi
    index="$1"
    if [ ! -f "$DATA_FILE" ]; then
        echo "Error: no items to complete"
        exit 1
    fi
    total=$(wc -l < "$DATA_FILE" | tr -d ' ')
    if [ "$index" -lt 1 ] || [ "$index" -gt "$total" ] 2>/dev/null; then
        echo "Error: index out of range"
        exit 1
    fi
    sed -i.bak "${index}s/^TODO/DONE/" "$DATA_FILE" && rm -f "${DATA_FILE}.bak"
    echo "Completed item $index"
}

if [ $# -eq 0 ]; then
    usage
fi

case "$1" in
    add)      shift; cmd_add "$@" ;;
    list)     cmd_list ;;
    complete) shift; cmd_complete "$@" ;;
    *)        usage ;;
esac
