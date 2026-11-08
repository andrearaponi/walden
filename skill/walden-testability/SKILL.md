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

## The Five Questions

Ask all five of every acceptance criterion, in requirements order. Every entry answers all five, each answer short enough for one cell of the matrix.

### 1. Stimulus: what must a proof provoke?

Read it off the criterion's EARS form:

| Form | What the proof must produce |
| --- | --- |
| Ubiquitous | no trigger: the input domain over which the response must hold |
| Event-driven | the event, from the source that raises it: a call, a message, a request, a timer |
| State-driven | the state, established through a public path and held during the check |
| Optional feature | the configuration that includes the feature |
| Unwanted behavior | the undesired condition: invalid input, a failure, a timeout, an outage |
| Complex | the state, then the event inside it |

Name how the proof produces it: which call, which message, which input, which configuration. When producing it needs a source the proof does not control, that is a controllability gap.

### 2. Observation point: where does the proof see the response?

Take the cheapest rung that sees the response from outside, in this order:

1. a returned value or output;
2. an emitted event or message;
3. state read back through a public interface;
4. a persisted artifact read through its public path;
5. a rendered element.

A private field never counts as an observation point, and neither does any other internal of the implementation. A log counts only when the log is the response the criterion names. When no rung sees the response, that is an observability gap.

### 3. Oracle: what decides pass or fail?

Name one of five oracle kinds: a value stated by the criterion, a worked example, an invariant, a reference implementation, or a recorded fixture. The oracle must come from outside the implementation under test. A fixture recorded from the current implementation is an oracle only for characterization proofs that pin existing behavior, never for new behavior. An expected value that only the implementation's own formula could produce is an oracle gap.

### 4. Proof kind: which is the cheapest proof that separates success from failure?

Name exactly one, climbing the ladder only as far as the observation point requires: example, property, differential, contract, end-to-end, manual.

- An invariant in ubiquitous form defaults to a property proof, unless its domain is small enough to enumerate with examples: say which, and why.
- A rewrite with a trusted predecessor takes a differential proof.
- An owned boundary with an external system takes a contract proof.
- An end-to-end or manual proof always carries its reason: name the response that no cheaper kind can see.

### 5. Loop class: where in the loop does the proof run?

Name exactly one:

- **inner loop**: deterministic, and needing nothing outside the process. It fits the verification of every task.
- **checkpoint**: needs minutes or local services, such as a container, a database or a browser. It deserves a task of its own. A proof that starts a container is never inner loop.
- **manual**: needs a person or a live third party. Say that no Walden proof can cover the criterion, so the hole is visible before the task plan exists.

## Gaps And The Smallest Change

A gap is what keeps a criterion from a cheap proof. Report three kinds, each naming its cause:

- **Controllability gap**: the stimulus depends on a source the proof cannot drive: the clock, randomness, the environment, the network, the filesystem, concurrency, global state or an external service. Name the source. A source counts as controlled when the design injects it or the repository's test tooling already drives it, and then there is no gap.
- **Observability gap**: the response is visible only inside the implementation, such as a value held in a private field or a side effect that nothing records. Name where it stays hidden.
- **Oracle gap**: the only available expected value would be computed by the implementation under test. Propose an independent oracle. When only the requirement's owner can supply it, such as a rule nobody stated, the change is a question for the requirements phase.

Pair every gap with the smallest design change that closes it, trying the controls in this order:

1. **Platform control.** The control the platform's test tooling already gives, with no design change: a virtual clock in the standard test library, a local test server, a temporary directory, an environment override for one test.
2. **One seam.** A single seam carrying the missing role: a clock, a random source, a configuration value, or a port to the network, the filesystem or a service; for an observability gap, a returned result or an emitted event.
3. **Restructuring.** Separating decision from effect, only when neither of the above reaches the stimulus or the response.

No gap, no change: a criterion without a gap gets no seam, no interface and no injection. Every proposal cites the design section or the `file#symbol` it changes. When the repository's language is identified, show the change in that language's idiomatic form, taken from the seams reference.

**Unverifiable criteria are not gaps.** When an acceptance check states no observable distinction between success and failure, such as a judgment nobody stated, report the criterion as unverifiable, propose no design change, and route it to the walden skill's requirements phase.

## Recipe: Review

Inputs: the approved requirements, the design body, `.walden/constitution.md`, the build manifests and the footprint. The manifests give the language, its version and its test tooling: the Go version, for instance, decides whether `testing/synctest` exists.

The footprint is the files and symbols the design names, plus the functions they call directly. Open nothing else: the cost of a review follows the number of criteria and the locations the design names, not the size of the repository. Run the search hints of the seams reference inside the footprint only, and read every hit before reporting it. A location the design names that does not exist is a design defect at that anchor; never substitute a guessed file.

1. Run the Entry Check: it sets the mode and the write regime.
2. List the acceptance criteria in requirements order, with their IDs.
3. Answer the five questions for each criterion: one entry per criterion, citing its ID. In a review of the drafted design every entry also cites a design section or a `file#symbol` anchor; in design-input mode the entries cite the requirements only and say what the design must provide: the stimuli, the observation points, the oracles and the seams.
4. Collect the gaps, each with its smallest change. Where the design changes existing code, every controllability gap caused by that code cites its file and symbol.
5. Characterization first: when the design changes a symbol that no test and no Walden proof command exercises, propose characterization proofs that pin its current outputs before the change, and name the search behind the claim that nothing exercises it.
6. Report: the mode label, the matrix, the gap lines and the next step.

## The Testability Matrix

One row per acceptance criterion, in requirements order, always with exactly these columns:

| Criterion | Stimulus | Observation | Oracle | Proof | Loop | Gap |
| --- | --- | --- | --- | --- | --- | --- |

Below the table, one line per gap: the criterion, the gap, the proposed change with its anchor, and its state: proposed, accepted, or rejected with the user's reason. Keep the cells short; the detail belongs to the gap lines. Use only criterion IDs present in the requirements: the validator rejects unknown IDs.

## The Anchoring Ceremony

The only write. Preconditions: the CLI reads the design as `draft`, and the user accepted the review or named which changes are accepted and which are rejected. Silence is not acceptance.

1. Write the matrix and its gap lines under the sub-heading `### Testability Matrix` at the end of the Verification Plan section. If the sub-heading exists, replace everything under it: a new run replaces the matrix and never appends a second one.
2. Write each accepted change into the design section it cites.
3. Record each rejected change as one entry in Failure Modes And Tradeoffs naming the criterion, the open gap and the user's reason.
4. Leave the frontmatter byte-identical: edit the body only, never `status`, timestamps or fingerprints.
5. Run `walden validate <feature>`, which is read-only. If it fails, show the error and repair the written text before handing off; never leave an invalid design behind.
6. Hand the design back to the walden skill's review loop: designs are opened for review and approved there, never here.

## Output Standards

- Open every review with its mode label: **design input**, **review of the drafted design**, or **proposed revision** for a design in review or approved.
- One entry per acceptance criterion, each citing its ID; every gap with its anchor and one proposed change; unverifiable criteria listed apart.
- Name the language and version the proposals assume, and the files of the footprint.
- Testability is not correctness: the matrix says what can be proven cheaply, not that the proofs written later will be good.
- End with the next step and who takes it: accepting or rejecting the proposed changes, the walden skill's review loop, its reconcile and re-review path, or its requirements phase.
