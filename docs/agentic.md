# The Agentic Flow

Most teams do not hand-write EARS criteria — their coding agent does. Walden is built for exactly that division of labor: the **skill** authors, the **CLI** enforces. This page is how the two halves compose into the flow you actually run: one prompt in, a reviewed spec tree out, every gate passed, every proof recorded — nothing typed but the intent.

## The division of labor

Walden splits every feature into work that needs judgment and work that needs enforcement:

| Non-deterministic — the skill (or a human) | Deterministic — the CLI |
| --- | --- |
| Drafting requirements and acceptance criteria | Validating structure, traceability, freshness |
| Designing architecture, weighing alternatives | Opening and sealing review gates |
| Generating the implementation plan | Running proofs, recording evidence |
| Writing the code | Deriving states, judging releasability |
| Deciding when to re-plan | Reconciling stale chains |

The boundary is absolute in both directions: **the CLI never authors content, and the skill never mutates workflow state**. Every status flip, approval seal, checkbox, and evidence record goes through a command — so the guarantees in [the lifecycle](lifecycle.md) hold identically whether a human or a model did the typing. Trust comes from the gates, not from the model.

## Install the skill

The skill ships embedded in the binary, versioned with it:

```bash
walden skill install claude     # or: codex | copilot | opencode | --all
walden skill status             # drift against the embedded copy
```

`walden update` re-syncs every installed skill with the binary, so the instructions and the CLI they drive never diverge. `--project` installs at repository scope instead of user scope.

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
- Evidence doesn't care who typed the code: `verified` means the proof passes on the tree in front of you, today.
- Every agent action that matters is a CLI invocation — auditable in the same JSON envelope your pipelines already parse.

Onboarding a team is therefore one command and one habit: `walden skill install`, and "let's use Walden" at the start of the session. The ceremony costs the agent, not the human — the human keeps the judgment calls.

## Companions and internals

- **`walden skill show`** prints the embedded `SKILL.md` — the exact operational instructions the agent follows. It is intentionally self-contained and is the agent-facing counterpart of these human-facing docs.
- **`walden-history`** (shipped in-repo at `skill/walden-history/`, installed manually) narrates committed `.walden/` history — sourced feature chronicles, product eras, rework archaeology — and officiates the [retirement ceremony](adoption.md#retirement).
