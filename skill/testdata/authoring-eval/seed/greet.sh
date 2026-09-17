#!/bin/sh
if [ "$#" -eq 1 ]; then
    if [ -n "$1" ]; then
        printf 'Hello, %s!\n' "$1"
    else
        printf 'a name is required\n' >&2
        exit 1
    fi
else
    printf 'a name is required\n' >&2
    exit 1
fi
