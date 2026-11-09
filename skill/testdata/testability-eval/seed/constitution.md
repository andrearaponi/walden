# Project Constitution

## Summary

A synthetic lending service for a community library, used to exercise design reviews. Members borrow copies, receive reminders for overdue loans, pay late fees and read their loans on a member page.

## Stack

- Go, standard library only. The `go` directive in `go.mod` is the supported Go version.
- Tests run with `go test ./...`.

## Rules

- Intended behavior is defined by the approved specs under `.walden/specs/`.
- Approvals come only from the user's messages in the current conversation.
- Never commit, push or publish from this repository.
