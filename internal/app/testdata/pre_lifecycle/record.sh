#!/bin/sh
# Regenerates recorded/ from portfolio/ with a released walden v0.14.5 binary.
# The recorder refuses any other release: see README.md beside this script.
#
# Usage: internal/app/testdata/pre_lifecycle/record.sh /path/to/walden
set -eu
binary="${1:?usage: record.sh /path/to/the/walden/v0.14.5/binary}"
cd "$(dirname "$0")/../../../.."
WALDEN_PRE_LIFECYCLE_BINARY="$binary" go test -count=1 -run '^TestRecordPreLifecycle$' ./internal/app
