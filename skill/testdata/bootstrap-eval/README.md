# Bootstrap decision fixtures

Seven candidate-only synthetic cases exercise CLI reuse, home/PATH resolution, consent and refusal, pinned binary-only installation, failed/misleading installation, and ambiguous skill ownership. This is not a generated-product A/B.

The actual final guide must be supplied in the recorded prompt. Every case has isolated HOME, PATH, agent directories and temporary storage; real user binaries/credentials must not satisfy a fixture prerequisite or be copied into it. CLI/download/install seams log actual calls and block real network requests. The successful installer path uses the unmodified installer with locally supplied fixture release payloads/checksums. Wrong-version and unavailable cases are explicitly faulted controls, not successful installations.

Record every turn, tool invocation, fixture snapshot, version/path check and manual criterion assessment. A passing transcript-reader test does not independently interpret arbitrary natural language. Missing, denied, failed or not-run evidence must not be relabeled as a pass.

Expected checks:
- reuse-compatible/home-discovered/verified-path: the selected usable CLI is queried and reused, not installed again or shadowed by an old PATH entry;
- consent-requested/no-preconsent-download/denial-respected: proposal precedes execution and refusal stops it;
- pinned-binary-only: the planned official v0.10.2 installer is used with matching --version and --no-skill after consent, with no integrity bypass;
- postcheck-compatible-path/postcheck-rejected/failure-stops-workflow: actual executable version/path is checked; missing/incompatible output or a failed download never authorizes workflow work;
- external-owner/ownership-decision/no-native-resync: Skills CLI-owned guides are not reinstalled by Walden and ambiguous overlapping copies are not automatically changed;
- no-install/no-unrequested-writes: no unrequested download/install, source/spec/ledger mutation, persistent shell edit, skill/registry rewrite, commit or publication.

Limits: seven sessions, at most eleven turns; at most three active minutes and a requested $0.50 client cap per session including follow-ups, with a $5 aggregate envelope including request-boundary headroom. No automatic retry or additional model preflight. A required failure/not-run result blocks acceptance and needs a user decision before further calls. No public listing/telemetry activity, real global install or adopter operations.

These observations cover bootstrap decisions only. Deterministic installer tests separately exercise platform, checksum, flag and native-mode permutations. Old authoring/integrity reports retain their own identities and are not replaced by this fixture set.
