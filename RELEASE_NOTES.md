## Walden v0.7.2

One fix, found the honest way: the first real project built on Walden broke its own CI on day one — and the failure was ours.

### What was broken

`walden repo init` pins the CI workflow it generates to the version of the binary that generated it (a v0.7.1 hardening). It pinned **verbatim**: binaries built from a clone carry git-describe versions like `v0.7.1-2-gabc1234`, which `go install` cannot resolve. The generated workflow failed its very first run with `invalid version: unknown revision`.

### The fix

The pin is now an allowlist of the only installable shape — strict `vMAJOR.MINOR.PATCH` release tags. Everything else (dev builds, describe suffixes, pseudo-versions) falls back to `@latest`, exactly what the workflow shipped before pinning existed.

Release binaries always stamped real tags and were never affected. If a clone build generated a broken pin in your repository, `walden repo init` refreshes it.

### Upgrading

```bash
walden update
```

That's the whole ceremony now.
