# Project Constitution

## Summary

A synthetic local greeting CLI used to exercise authoring behavior.

## Stack

POSIX shell, no external dependencies or network services.

## Rules

- Intended behavior is defined by the approved greeting spec.
- Contract-preserving maintenance needs tests, not a new spec, unless the user explicitly requests one.
- Run `sh tests/test_greet.sh` after implementation changes and refresh affected Walden evidence.
- Approvals and implementation authorization must come from the current user's messages.
- Never commit or publish from this fixture.
