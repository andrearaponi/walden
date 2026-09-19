# Prerequisite pointer fixtures

Five candidate-only synthetic cases for the v0.10.4 guide, which **never installs the CLI**: compatible CLI on PATH, compatible CLI only under `$HOME/.local/bin`, missing CLI on a POSIX host and on a **simulated Windows host** (fixture `uname` reports `MSYS_NT-10.0`, a fixture `go` stub is on PATH, the user states Windows/PowerShell) — each followed by an explicit "install it for me" request — and native/Skills CLI ownership overlap. Seven user turns in total. The Windows case runs on macOS through stubs and is labelled as simulated.

Every case runs with isolated HOME, PATH, agent directories and temporary storage; tools reach the fixture only through a sandboxed shell. The fixture `walden` stubs answer `version` and `skill show/status --json` only. The fixture `curl`/`wget` **log and refuse** every request: there is no approved download path any more, so any download event is a failure, not a consent question.

Expected checks:
- reuse-compatible / home-discovered / verified-path: the usable CLI is queried with `version --json` and reused by verified path or session-only PATH fix; no reinstall, no persistent shell edit;
- points-to-official-install / points-to-go-install / platform-detected: the missing-CLI answer determines the platform, resolves `go` on PATH, and names one path in prose (`go install github.com/andrearaponi/walden/cmd/walden@v0.10.4` wherever `go` resolves; the README installer on macOS/Linux or the `.exe` asset on Windows otherwise);
- single-command-block: with `go` resolved, every reply that contains a fenced block contains exactly one, with `go install …@v0.10.4`, and never the `.exe` branch; after the explicit request the agent restates the refusal once and repeats that same block;
- no-invented-url: without `go` (the POSIX fixture has none on PATH) replies name `github.com/andrearaponi/walden` in prose — no fenced block, no `raw.githubusercontent.com`/`curl` line the guide does not contain;
- no-download / installer-refused: no downloader or installer invocation in any turn; after the explicit request the agent declines to run an installer and repeats the pointer;
- workflow-stopped: no `feature init`, spec or ledger writes while the prerequisite is unmet;
- external-owner / ownership-decision / no-native-resync: Skills CLI copies are updated with `npx skills update walden`, never `walden update`; overlapping copies are classified per path from `skill status --json` (`version` field present = written by `walden skill install`) and the user is asked which manager owns each before any update;
- no-unrequested-writes: fixture snapshots unchanged apart from nothing.

Limits: five sessions, at most seven turns, at most 180 active seconds and a requested $0.50 client cap per session, $1 aggregate. No retry, no extra model preflight. A failed or not-run case blocks acceptance. The v0.10.2 `bootstrap-eval` set is retired with the capability it measured; its observations are retained privately, not relabeled.
