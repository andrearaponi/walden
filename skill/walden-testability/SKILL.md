---
name: walden-testability
description: "Walden Testability reviews a Walden design for testability before the task plan exists. For every acceptance criterion it asks what a proof must provoke (controllability), where it sees the response from outside (observability), which oracle decides pass or fail without trusting the implementation, which proof kind is cheapest and where it runs in the agent's loop, then proposes the smallest seam that closes each gap. Use when the user asks for a testability or verifiability review of a Walden design, asks how a criterion can be proven or what its proof will cost, or raises controllability, observability, proof cost, flaky or slow proofs, or design for testability at the design phase. It advises the design and never advances the workflow: approvals stay with the walden skill."
metadata:
  short-description: Walden design testability review
---

# Walden Testability

Use this skill to settle what each proof will cost while the design is still being written. A proof is cheap when the design lets it provoke the condition a criterion names and see the response from outside. When the design does not, the proof becomes slow, flaky, end-to-end only or impossible, and an agent working in a loop learns to work around it instead of fixing the code.

For every acceptance criterion this skill asks whether a proof can provoke the condition its EARS clauses name (controllability), see the response its SHALL names from outside (observability), decide pass or fail against an oracle the implementation did not produce, and run inside the agent's loop. It answers with the cheapest sufficient proof and the seams the design must provide.

The walden skill authors the present, walden-history reads the past, and this skill makes the promised provable. It advises the design; it never advances the workflow. Reply in the user's preferred language.

## When To Use

- The user asks for a testability or verifiability review of a Walden design, or of one feature's acceptance criteria.
- The user asks how a criterion can be proven, which proof it needs, or what that proof will cost.
- Controllability, observability, oracles, flaky or slow proofs, or design for testability come up while a design is being drafted.

Do not use it to author requirements, design or tasks, to open or approve a review, or to write or run tests: that is the walden skill.

## Prerequisites

- **The walden CLI.** Run `walden version` before anything else. If no walden executable answers, stop: produce no review, and point the user to the install instructions in the README at https://github.com/andrearaponi/walden. This skill never installs the CLI.
- **An approved contract.** A feature under `.walden/specs/<feature>/` whose requirements the CLI reports approved and fresh. The Entry Check decides.

## Product Boundary

- **One write path.** The only write path is the body of a draft design, and only after the user accepts the review or names the changes to accept and to reject: see The Anchoring Ceremony. Frontmatter, requirements, tasks, the constitution, lessons and evidence are never written.
- **State from the CLI.** Workflow state comes only from `walden status <feature> --json`, never inferred from frontmatter text.
- **Sealed designs.** A design in review or approved is never edited. Present the findings as a proposed revision for the walden skill's reconcile and re-review path, and leave `design.md` byte-identical.
- **No approvals.** A request to author, open for review or approve a design, or any other Walden document, is declined and routed to the walden skill. This skill runs no `walden review` and no `walden reconcile` command.
- **No tests.** This skill neither writes nor runs tests, and it executes no code from the repository.

## Entry Check

1. Run `walden version`. No executable: stop, as Prerequisites says.
2. Run `walden status <feature> --json` and read each document's `status` and `fresh`. If the command fails, quote its error verbatim and run no review.
3. Unless `requirements.md` reads `approved` with `fresh: true`, decline the review: write no entries, say why, and route the user to the walden skill's requirements phase to finish, approve or reconcile the requirements.
4. The design's `status` sets the write regime: `draft` allows The Anchoring Ceremony after acceptance; `in-review` or `approved` allows findings only, presented as a proposed revision.
5. The design body sets the mode. When every required section still holds only its scaffold placeholder, the review is **design input**: its entries cite the requirements only and state what the design must provide. Otherwise it is a **review of the drafted design**: every entry cites a design section or a code location the design names.
