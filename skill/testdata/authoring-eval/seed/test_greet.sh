#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
test "$(sh "$root/src/greet.sh" Ada)" = 'Hello, Ada!'
if sh "$root/src/greet.sh" '' >/dev/null 2>&1; then
    echo 'empty name was accepted' >&2
    exit 1
fi
if sh "$root/src/greet.sh" >/dev/null 2>&1; then
    echo 'missing name was accepted' >&2
    exit 1
fi
printf 'PASS: greeting-contract\n'
