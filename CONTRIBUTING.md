# Contributing to Walden

Thank you for your interest in contributing to Walden.

## Getting Started

1. Fork and clone the repository.
2. Ensure Go 1.25.0 or later is installed.
3. Run the test suite:

   ```bash
   go test ./...
   ```

## Development Workflow

Walden follows its own spec-driven workflow. For non-trivial changes:

1. Create a feature spec with `walden feature init <name>`.
2. Draft requirements, design, and tasks.
3. Get approval before implementing.

For small fixes and improvements, a pull request with clear context is sufficient.

## Engineering Standards

- **TDD-first**: new behavior starts with a failing test.
- **Fail-fast**: commands reject blocked states before partial writes.
- **Minimal production code**: write the minimum needed, test thoroughly.
- **Shell-safe verification**: use structured argv proofs, not shell-interpreted strings.

## Pull Requests

- Keep PRs focused on a single change.
- Include tests for new behavior.
- Ensure `go test ./...` passes before submitting.
- Describe what changed and why in the PR description.

## Reporting Issues

Use GitHub Issues to report bugs or request features. Include:

- What you expected to happen
- What actually happened
- Steps to reproduce
- CLI version (`walden version`)

## Code of Conduct

Be respectful and constructive. We are building in the open and value honest, direct communication.

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
