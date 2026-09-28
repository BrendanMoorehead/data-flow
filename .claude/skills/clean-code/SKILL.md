---
name: clean-code
description: Coding standards for this repo (comments, function size, naming, reuse). Use before writing, editing, or reviewing any code in data-flow.
---

# Clean code standards

These rules apply to every change in this repo. When a rule conflicts with
getting something working, follow the rule and write the code again.

## 1. Let the code explain itself

Before writing a comment, try to make it unnecessary:

- Rename the variable or function so it says what it holds or does.
- Pull the confusing expression into a well-named function or variable.
- Replace magic numbers and strings with named constants.

Write a comment only when the code is still hard to follow after that, such
as a complex algorithm or a subtle ordering requirement. The comment then
explains **what** the tricky code does.

Comments must never:

- mention removed, old, or alternative code ("used to…", "previously…",
  "we tried X")
- tell the story of a decision ("we chose this because…"). Decisions go in
  `docs/decisions.md` and the README, not in the code.
- repeat what the code already says (`// increment count`)
- be left as commented-out code. Delete it; git keeps the history.

## 2. Make functions atomic

- A function does one thing, at one level of abstraction.
- If you describe a function and the description contains "and", split it.
- Keep functions short. If a block needs a heading comment, it should be its
  own function named after that heading.
- Prefer few parameters. When the parameters travel together, group them into
  one named type.
- Separate commands from queries. A function either changes state or returns
  information, not both.
- Keep side effects (I/O, clock, randomness, network) at the edges, so the
  core logic stays pure and easy to test.

## 3. Names explain themselves

- Variables and types are nouns that say what they hold:
  `staleAfterSeconds`, not `t`. `providerEventId`, not `id2`.
- Functions are verbs that say what they do: `normalizeMoneyline`,
  `isStale`, `mergeObservation`.
- Booleans read as questions: `isStale`, `hasFailedRecently`.
- Units go in the name when they aren't obvious: `latencyMs`, `observedAt`.
- Use the same word for the same concept everywhere. Pick one of
  "observation", "quote", or "price" and use it consistently.
- Abbreviate only when the abbreviation is universal, like `id` or `url`.

## 4. Keep looking for reuse

- Before writing new code, search for an existing function that does this or
  nearly this. Extend or extract it instead of duplicating.
- When the same logic shows up a second time, extract it into a shared,
  well-named function.
- Reuse only where the pieces mean the same thing. Two blocks that look alike
  but change for different reasons should stay separate. The wrong
  abstraction costs more than a little duplication.
- When you notice existing code that breaks these rules while you're working
  nearby, clean it up without changing its behavior.

## 5. General clean-code habits

- Return early instead of nesting conditionals deeply.
- Make illegal states hard to represent. Use types and enums instead of free
  strings.
- Fail loudly at boundaries with specific errors. Never swallow an error
  silently.
- Leave the code cleaner than you found it.
