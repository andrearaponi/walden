# Quickstart

One real feature, from an empty repository to a releasable verdict — by hand, so you see every moving part. In daily use you will rarely type most of this: [the agentic flow](agentic.md) has your coding agent author the documents and drive these same commands, stopping at each gate for your approval. Everything below is what actually happens underneath.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
walden version
```

If the shell answers `command not found: walden`, `~/.local/bin` is not on your PATH yet. The installer prints the line to add; put it in your shell profile to keep it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Alternatively, install the guide first with Skills CLI:

```bash
npx skills add andrearaponi/walden --skill walden
```

The guide requires CLI v0.10.4 or a newer compatible release and never installs it: if the binary is missing or incompatible the skill stops, detects your platform and points you to the installer above, to `go install github.com/andrearaponi/walden/cmd/walden@v0.10.4` (Windows with Go), or to the `walden-v0.10.4-windows-<arch>.exe` asset on GitHub releases. Update the guide with `npx skills update walden` and the binary with `walden update`; neither touches the other. See the guide's [CLI Prerequisite](../skill/walden/SKILL.md#cli-prerequisite) section.

## Initialize

```bash
git init my-project && cd my-project
walden repo init
```

This creates `.walden/` (a `constitution.md` for stable project context, a `lessons.md`, a scoped `.gitignore`) and two files under `.github/`: a pull request template and a generated CI workflow (`workflows/validate-walden.yml`) pinned to the version that generated it.

```bash
walden feature init "Greeting Service"
```

Feature names normalize to kebab-case: the spec lives in `.walden/specs/greeting-service/` as three draft documents — `requirements.md`, `design.md`, `tasks.md`.

## Phase 1 — Requirements

Edit `.walden/specs/greeting-service/requirements.md`. Keep the frontmatter; replace the template body:

```markdown
# Requirements Document

## Introduction

A command that greets a named user, as the smallest end-to-end slice of the CLI.

## Requirements

### R1 Greeting

**User Story:** As a user, I want a greeting by name, so that I know the CLI sees my input.

#### Acceptance Criteria

1. `R1.AC1` WHEN the user runs `greet <name>`, the system SHALL print `Hello, <name>!`.
   - Acceptance check: the output includes the supplied name in the exact greeting.
2. `R1.AC2` IF `<name>` is empty, THEN the system SHALL exit non-zero naming the missing argument.
   - Acceptance check: an empty name produces the named error instead of a successful greeting.

## Non-Functional Requirements

- `NFR1` The command SHALL complete in under one second. (bridged by `R1.AC1`)

## Constraints And Dependencies

- `C1` Standard library only.

## Out Of Scope

- Localization.
```

Validate, then walk the review gate:

```bash
walden validate greeting-service
walden review open greeting-service --phase requirements
walden review approve greeting-service --phase requirements
```

Approval does more than flip a status: it records a SHA-256 **fingerprint of the approved body** in the frontmatter. From now on, any edit to this content is detectable — and stales everything built on top of it. This is the first link of the [approval chain](lifecycle.md#the-seal).

## Phase 2 — Design

Fill the six sections in the generated `design.md`: Architecture, Options Considered, Simplicity And Elegance Review, Failure Modes And Tradeoffs, Verification Plan, and Requirement Coverage. Additional sections are optional; do not invent an alternative or irrelevant detail just to fill a template. Coverage and verification must still be substantive. The validator checks structure, not the quality of these decisions.

For this feature, keep the frontmatter and replace the template body:

```markdown
# Feature Design

## Architecture

A single Go package with one function, `Greet(name string) (string, error)`, and a `main` that passes the first argument to it, prints the result and exits non-zero on error.

## Options Considered

A `--name` flag instead of a positional argument: rejected, because the requirement names `greet <name>`.

## Simplicity And Elegance Review

One pure function holds the rule. `main` only wires the argument and the exit code, so the rule can be tested without running a process.

## Failure Modes And Tradeoffs

An empty name returns an error that names the missing argument, and the process exits non-zero. There is no other failure: the command does no I/O beyond standard output.

## Verification Plan

A Go test, `TestGreet`, calls `Greet` with a name and with an empty string, and checks the exact greeting and the error.

## Requirement Coverage

| Requirement | Covered By |
| --- | --- |
| `R1` | `Greet` and `TestGreet` |
| `NFR1` | `Greet` does no I/O; `TestGreet` runs in milliseconds |
```

Then:

```bash
walden validate greeting-service
walden review open greeting-service --phase design
walden review approve greeting-service --phase design
```

The design's approval binds to the requirements' fingerprint. If requirements change later, the design is stale by construction, not by convention.

## Phase 3 — Tasks

Edit `tasks.md`, again keeping the frontmatter: a two-level plan where every leaf task names the acceptance criteria it implements, the design section it follows, and — the part that matters — an executable **proof**:

```markdown
# Implementation Plan

- [ ] 1. Greeting command
  - [ ] 1.1 Implement greet with argument validation
    - Requirements: `R1.AC1`, `R1.AC2`, `NFR1`
    - Design: Architecture
    - Verification:
      - command: ["go", "test", "-v", "-count=1", "-run", "^TestGreet$", "./..."]
        expect_output: "--- PASS: TestGreet"
        covers: ["R1.AC1", "R1.AC2"]
```

```bash
walden validate greeting-service
walden review open greeting-service --phase tasks
walden review approve greeting-service --phase tasks
```

## Execute

The proof above is a Go test, so this example needs Go. With another stack, change the proof command and write the equivalent test: a proof is any command whose exit code and output tell success from failure.

Create the module:

```bash
go mod init example.com/greet
```

Save the code as `greet.go`:

```go
package main

import (
	"errors"
	"fmt"
	"os"
)

// Greet returns the greeting for name, or an error naming the missing argument.
func Greet(name string) (string, error) {
	if name == "" {
		return "", errors.New("missing argument: name")
	}
	return "Hello, " + name + "!", nil
}

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	greeting, err := Greet(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(greeting)
}
```

And the test as `greet_test.go`:

```go
package main

import "testing"

func TestGreet(t *testing.T) {
	got, err := Greet("Ada")
	if err != nil || got != "Hello, Ada!" {
		t.Fatalf("Greet(%q) = %q, %v", "Ada", got, err)
	}
	if _, err := Greet(""); err == nil || err.Error() != "missing argument: name" {
		t.Fatalf("Greet(\"\") error = %v, want the named missing argument", err)
	}
}
```

Then complete the task — the CLI runs the proof and refuses the completion if it fails:

```bash
walden task status greeting-service     # readiness and next runnable task
walden task complete greeting-service 1.1
```

A passing completion checks the box **and** writes a record to `.walden/evidence/greeting-service.json`: the proof's steps and outcome, bound to the approved spec fingerprints and to a digest of the working tree. "Done" is now a claim you can re-prove.

## Trust, later

Weeks pass, code changes. Ask what still holds:

```bash
walden evidence status greeting-service   # derived states and diagnostic profile probes
walden verify greeting-service            # re-runs proofs for anything no longer verified
```

States are **derived at read time** — `verified`, `delivered`, `stale-spec`, `stale-code`, `failed`, `unattested`, `unrecorded`, `pending`. `delivered` belongs to a plan that declares a [delivery proof](lifecycle.md#delivery-proofs); this one declares none. The view separates contract binding, code freshness and execution provenance. Use `walden adopt greeting-service` for a legacy assessment without proofs or probes; a recovered hash cannot invent missing historical purity. Choose the current contract scope before replay, and aggregate checks at declared batch/delivery checkpoints rather than after every edit.

## Certify

The verdict names a commit and refuses uncommitted changes outside `.walden/`, so commit first:

```bash
git add -A
git commit -m "Greeting service"
walden release check
```

One deterministic verdict over the reported scope: approved fresh chains, validation, resolved decision markers, current execution assurance, pending work completed or explicitly waived, and the worktree policy. The gate executes no proofs or profile probes and writes nothing. Add `--strict` to bind the exact judged spec/evidence inputs to the named commit, even when ignored. A named-feature pass is not a repository-wide certificate; non-strict mode does not attest that local metadata is committed.

```text
Summary: RELEASABLE — 1 feature(s) certified [portfolio: greeting-service], commit 3f2a91c40d77
```

## The same loop, driven by an agent

Everything you just did by hand is what the AI skill does for you. Install it once with the Skills CLI —

```bash
npx skills add andrearaponi/walden --skill walden
```

— then open your agent and state the intent ("We need a small todo CLI… let's use Walden"). The skill drafts each document, runs the validations, and stops at every gate for *your* approval; the CLI seals the decisions and runs the proofs exactly as above. Same gates, same evidence, none of the typing: [The Agentic Flow](agentic.md).

## Where to next

- How the skill and the CLI divide the work: [The Agentic Flow](agentic.md)
- The mechanism behind each step: [The Spec Lifecycle](lifecycle.md)
- The full command loop, including reconciliation and lessons: [The Daily Workflow](workflow.md)
- An existing repository with specs that predate Walden's current contract: [Brownfield Adoption](adoption.md)
