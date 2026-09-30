---
name: decision-checkpoints
description: How to handle design decisions in data-flow. Use before planning or starting any feature, and whenever a choice comes up that a reviewer might ask "why?" about.
---

# Decision checkpoints

Design decisions in this repo belong to Brendan. The AI's job is to spot a
decision, lay out the options, and recommend one. It does not make the
decision.

## What needs a checkpoint

Stop and ask before deciding anything a reviewer might ask "why?" about:

- the stack: language, framework, storage
- the canonical data model and how provider data maps into it
- how records are matched across providers (identity)
- conflict, ordering, duplicate, and freshness rules
- retry, backoff, and failure semantics
- the API's shape and what metadata it exposes
- scope: what's in, what's cut, which upstream problems get simulated
- anything the README will state as a trade-off

Things that don't need a checkpoint, as long as they follow the
`clean-code` skill: naming, splitting functions, local refactors, test
structure inside a decided design.

## How to raise a checkpoint

Keep each one short:

1. **The decision.** One line saying what has to be chosen.
2. **The options.** Two or three of them, each with its main trade-off.
3. **A recommendation**, and why.
4. **What it affects.** Which part of the brief or later work depends on it.

Group related decisions together. Raise the ones that block progress first.
Then wait for Brendan's answer before building on it.

## After a decision

- Add a row to `docs/decisions.md` with what was decided and Brendan's reason
  in his own words. If he gave no reason, leave "Why" empty. Never invent
  one.
- Record the AI's involvement: none, accepted, reworked, or rejected. The
  "AI & Tools" note needs at least one real case of AI output being changed
  or rejected.
- If a request goes against the BE-01 brief, say so out loud before acting
  on it.
