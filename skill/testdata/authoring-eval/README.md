# Authoring acceptance fixtures

These are synthetic inputs and a fixed comparison rubric, not a runtime skill dependency or an automated judge of model behavior.

## Prepare a pair

1. Freeze the baseline skill/scaffolds from commit `3995cab96c79f2de891f7c86ed559d48d3a7253a` before editing. Retain the v0.10.1 executable independently of the active installation.
2. Build each variant's binary outside its source tree. Use its own binary for both scaffolding and skill installation.
3. Create a fresh temporary git repository per variant/scenario. Run `walden repo init` and `walden feature init greeting`.
4. Copy `seed/greet.sh` to `src/greet.sh`, `seed/test_greet.sh` to `tests/test_greet.sh`, and the filled constitution to `.walden/constitution.md`. Replace only the three scaffold bodies with `seed/{requirements,design,tasks}.md`; retain CLI-created frontmatter.
5. Prime this synthetic fixture through review open/approve for each phase and complete its task through the CLI. These fixture setup operations are not approvals for the Walden project's work.
6. For the `broken` seed, change only the greeting prefix in the source to `Hi` after completion, so the implementation contradicts the approved contract and evidence becomes stale.
7. Install the variant with its binary into an existing project-scoped host (`claude` or `codex`). Keep real user-global skill files unchanged. Ensure the session loads the intended project skill, not a different global copy.

Fixture source files deliberately do not live under a directory named `.walden`, which the development repository ignores. Generated fixture state lives only in temporary repositories.

## Run and review

Use the same host/version/model/settings and the same `scenarios.json` prompt and follow-up sequence for both variants. Start a fresh session for every pair member. Supply follow-ups as actual user messages after the corresponding response; never place future approvals in an initial agent prompt. Record CLI/tool actions and generated files, not just final answers.

The five scenarios exercise contract-preserving maintenance, an explicit policy override, new-feature phase gates, a meaningful scope correction, and unresolved intent. The ordinary small-feature summary and corrected-contract summary provide the quiet/logged lesson pair.

Inspect each required criterion against its scenario's `expect` list. Content reduction and the generated heading set also use the inspected bundle/artifacts; their inclusion in the comparison is not a claim that transcript parsing can prove their semantics. A baseline failure is valid comparison data. A missing or failed candidate observation blocks behavioral acceptance.

## Local report

Keep reports and transcripts under `temp/skill-authoring-quick-wins/<run-id>/`. The final report used by the task proof is `acceptance/report.json`. Do not publish private run data or copy this directory into the product.

The small test-only report contract is defined by `authoringReport` in `skill/authoring_eval_test.go`:

- `kind`: `observed` for real reviewed sessions, never for fabricated/synthetic unit-test data.
- `baseline_commit`, `agent`, `agent_version`, `model`, `settings`: explicit provenance for the paired runs.
- `baseline` and `candidate`: SHA-256 maps for `skill/walden/SKILL.md` and the three `templates/spec/*.md.tmpl` files. Candidate values must match the current source.
- `runs`: one entry per variant (`baseline`/`candidate`) and scenario. Each names its verdict, nonempty report-relative transcript/artifact paths, and a check for every criterion listed in that scenario.
- Each check has `id`, `verdict`, and an existing report-relative `anchor`, optionally with a `#L<number>` suffix. Paths must remain inside the report directory.

Use `pass`, `fail`, or `not-run` honestly. Acceptance rejects unexecuted runs, missing anchors, stale candidate hashes, or required candidate criteria that are not passed. The checker validates report integrity and declared observations; the reviewer remains responsible for evaluating the transcripts.

Run the explicit acceptance test with `WALDEN_AUTHORING_EVAL_REPORT` pointing to the real report. Without that input the ordinary suite reports a skip; it does not claim behavior was tested. Never substitute a synthetic passing report to turn this test green.
