# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Walden, please report it responsibly.

**Do not open a public issue for security vulnerabilities.**

Instead, use [GitHub private vulnerability reporting](https://github.com/andrearaponi/walden/security/advisories/new).

Include:

- A description of the vulnerability
- Steps to reproduce
- The potential impact
- Any suggested fix (optional)

## Response Timeline

- **Acknowledgment**: within 48 hours
- **Initial assessment**: within 7 days
- **Fix or mitigation**: as soon as practical, with a coordinated disclosure

## Scope

This policy covers the `walden` CLI binary and the public skill bundle shipped in this repository.

It does not cover:

- Third-party integrations or forks
- User-generated spec content
- Enterprise features that are not yet released

## General Guidelines

- Verification commands in examples and templates use wrapper scripts or structured argv to avoid shell injection.
- The CLI does not execute arbitrary user input as shell commands.
- Spec documents are treated as data, not executable code.
- The JSON output contract does not include secrets or credentials.
