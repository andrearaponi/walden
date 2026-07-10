## Walden v0.7.0

The last release you install by hand. From here on, updating Walden is one command:

```bash
walden update
```

### Self-update, fail-closed

`walden update` resolves the latest release (or a pinned `--version <tag>` — downgrades and rollbacks included), downloads the binary for your platform, and verifies its SHA-256 against the release's `checksums.txt` before anything touches your install. There is no bypass flag: an artifact that cannot be positively verified is never installed. The swap is atomic, and a post-install smoke test restores the previous binary if the new one fails to run — no path ends without a working executable.

### Skills follow the binary

An update re-installs every skill installation it finds, so your agents pick up the embedded skill matching the new binary. The drift `walden skill status` used to report after every upgrade now heals itself; when something cannot be re-synced, it degrades to a warning and `walden skill install <agent>` repairs it.

### Check mode for humans and agents

`walden update --check` reports what would change and writes nothing — exit 0 either way, with a machine-readable `update` block under `--json` (additive within `schema_version: v0beta1`):

```json
"update": {
  "current_version": "v0.6.0",
  "target_version": "v0.7.0",
  "update_available": true,
  "applied": false
}
```

### The kernel stays offline

Networking is confined to this one explicit command. No other command checks for updates, phones home, or touches the network — determinism, air-gapped CI, and agent pipelines stay exactly as they were. As a bonus, `go install` binaries now report their real version instead of `dev`.

### Upgrading (one last time by hand)

```bash
curl -fsSL https://raw.githubusercontent.com/andrearaponi/walden/main/install.sh | sh
# or: go install github.com/andrearaponi/walden/cmd/walden@v0.7.0
```

Next time, it's `walden update`.

### Built the Walden way

Specified and shipped through Walden's own gated ceremony: Requirements → Design → Tasks with 33 EARS acceptance criteria, 100% task and proof reference coverage, and every task completed through `walden task complete` with passing proofs — including a repo-wide proof that `net/http` stays confined to the update path.
