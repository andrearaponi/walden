## Walden v0.3.0

A small, focused release: the JSON output contract graduates from alpha to beta.

### Changed

- **JSON contract: `v0alpha1` → `v0beta1`.** After sustained real-world use, the contract is no longer "alpha" — its shape has settled. `v0beta1` signals a maturing contract you can build integrations on with care: it may still change before the stable `v1` (which will land at v1.0.0, with a documented migration guide), but breaking changes are flagged in the changelog. The envelope shape is unchanged from v0.2.0 — only the version label advances. Consumers should match the `v0` prefix rather than the exact string.

### Install

```bash
go install github.com/andrearaponi/walden/cmd/walden@v0.3.0

# or: build the binary and optionally install the AI skill
git clone https://github.com/andrearaponi/walden.git
cd walden && ./setup.sh
```

Pre-built binaries for linux/darwin × amd64/arm64 are attached to this release.

Still zero external dependencies.
