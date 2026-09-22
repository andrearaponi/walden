# The Agentic Flow

A coding agent can draft EARS criteria and drive Walden's commands: the **skill** authors, the **CLI** enforces the workflow. You still review the content, approve decisions, and authorize implementation. The agent reduces repetitive typing; it does not remove the cost of judgment.

## The division of labor

Walden splits every feature into work that needs judgment and work that needs enforcement:

| Non-deterministic — the skill (or a human) | Deterministic — the CLI |
| --- | --- |
| Drafting requirements and acceptance criteria | Validating structure, traceability, freshness |
| Designing architecture, weighing alternatives | Opening and sealing review gates |
| Generating the implementation plan | Running proofs, recording evidence |
| Writing the code | Deriving states, judging releasability |
| Deciding when to re-plan | Reconciling stale chains |

**The CLI owns workflow mutations.** The skill edits document bodies and invokes commands for status changes, approval seals, checkboxes, and evidence; it never writes those fields directly. Canonical templates come from `feature init`, not duplicated templates inside the skill. See [the lifecycle](lifecycle.md) for the mechanism.

## Choose the authoring lane

The skill first checks contract impact and project rules. A bugfix restoring approved behavior or a behavior-preserving refactor can stay outside a new spec cycle, while retaining tests and applicable evidence checks. New or changed intended behavior enters authoring at the appropriate phase. Unclear intent prompts a question before speculative documents are created. Explicit user or project requirements for a spec take precedence.

Within authoring, each AC gets a brief **Acceptance check** describing the observable distinction between success and failure, without choosing an implementation command. Design starts with six required headings; optional sections are added only when relevant. Human review still judges substance, including whether an alternative or assertion is meaningful.

## Install the skill

The skill is distributed through the [Skills CLI](https://skills.sh), separately from the binary:

```bash
npx skills add andrearaponi/walden --skill walden     # project-level; add --global for user scope
npx skills update walden                                # later
```

The guide declares the CLI version it requires and checks it through `walden version --json` before its first operation; the binary knows nothing about the guide. See [skill/walden/install.md](../skill/walden/install.md) for scopes, committed copies and removal.

## What a session looks like

You open your agent in a Walden-initialized repository and state the intent:

> We need a small, single-user todo app on the command line — add a task, list what's still pending, mark one done, all kept in a plain-text file, POSIX shell only. Let's use Walden.

From there the skill drives, and the CLI keeps score:

1. **Context first.** The skill reads `.walden/constitution.md` (your stack, conventions, hard rules) and `.walden/lessons.md` (past corrections) so it doesn't rediscover or repeat.
2. **Requirements.** It asks its clarifying questions, drafts `requirements.md` as EARS criteria, runs `walden validate` until clean — then **stops and presents the document to you**. Approval is yours: on your word it runs `walden review open` / `review approve`, and the CLI seals your decision with a fingerprint.
3. **Design, then tasks.** Same rhythm at each gate: draft, validate, present, wait, seal. Unresolved forks are parked as `[decision: …]` markers for you to settle — the release gate will not let them ship unresolved.
4. **Execution.** The skill implements task by task and completes each through `walden task complete`, which runs the proof and refuses anything that doesn't pass. Checkboxes are earned, not typed; evidence records land in the ledger with fingerprints, code identity, and the machine's profile.
5. **Certification.** `walden release check` renders the verdict. If it blocks, the blockers name their remedies and the skill works through them.

What's left behind is the spec tree from the [site's example](https://andrearaponi.github.io/walden/): three approved documents with sealed frontmatter, an evidence ledger, generated CI — and the same commands available to you, because the skill holds no private powers.

## What stays yours

The skill is deliberately constitution-bound at the points where judgment must be human:

- **Approvals.** The three gates seal *your* review, never the skill's own. It presents; you approve.
- **Waivers.** The skill never passes `--allow-pending` on its own — shipping less than the approved plan is a recorded human decision, reason included.
- **Decision markers.** `[decision: …]` forks are surfaced to you, not resolved silently.
- **Retirement.** Deleting superseded specs is confirmation-gated ceremony, never housekeeping.

## Why this favors adoption

The failure mode of agent-driven development is unverifiable velocity: plausible code, green-looking sessions, no durable claim about what was actually specified, reviewed, or proven. Walden inverts the economics — the agent supplies the speed, the kernel supplies claims that survive it:

- The fingerprint seals **what you approved**, not what the model generated afterward — post-approval drift is detected by construction.
- Evidence records declared proof outcomes against spec and code identities. Authors and reviewers remain responsible for choosing assertions that actually exercise the intended behavior.
- Every agent action that matters is a CLI invocation — auditable in the same JSON envelope your pipelines already parse.

Install the skill once with the Skills CLI, then ask to use Walden when appropriate. Human review still requires attention: the skill removes repeated scaffolding and state bookkeeping, not the responsibility to understand the contract.

## Companions and internals

- **[`skill/walden/SKILL.md`](../skill/walden/SKILL.md)** is the exact operational guide the agent follows. It is intentionally self-contained and is the agent-facing counterpart of these human-facing docs.
- **`walden-history`** (shipped in-repo at `skill/walden-history/`, installed manually) narrates committed `.walden/` history — sourced feature chronicles, product eras, rework archaeology — and officiates the [retirement ceremony](adoption.md#retirement).
