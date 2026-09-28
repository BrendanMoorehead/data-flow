# Design

This document describes how data-flow syncs sportsbook odds from several
imperfect providers into one canonical view, and how it behaves when those
providers fail. The reasons behind each choice are in
[decisions.md](decisions.md), referenced below as `#N`.

## Sources

Two books, **DraftKings** and **FanDuel**, are each reported by two
providers. That gives three adapters (#11, #14). Every feed is simulated. The
schemas are invented and don't reproduce either book's real API.

| Source | Kind | Schema traits | How off the board is shown |
|---|---|---|---|
| `aggregator` | Third-party paid API, covers both books | Decimal odds, its own event IDs and team spellings, paged, rate limited, lags behind the books | The market is left out of a snapshot (#16) |
| `draftkings_direct` | Direct feed | American odds, provider timestamp, supplies its own main-line flag | Explicit `suspended` status |
| `fanduel_direct` | Direct feed | A different shape from DraftKings', **no timestamps** (#32) | Explicit `suspended` status |

Every source is polled as a **full refresh** (#33). Each adapter splits its
provider into **slices** that match how that provider's site divides its data,
for example by league page or by game (#35).

## Lifecycle of a record

```
simulator (local HTTP mock servers, one per provider)       #37
  │
  ▼
poller: each slice polled on its own schedule and backoff   #36
  │     shared request budget per provider
  ▼
adapter.Poll(slice) ─ fetches and parses the provider schema
  │
  ▼
adapter maps to []Observation ◀── canonicalization point: data becomes ours here
  │     (canonical units: American odds, our book/market/side enums)
  ▼
identity ─ provider team names + event ID → our event ID     #12
  │        unknown team alias → observation rejected
  ▼
sanity checks ─ failures go to quarantined_observations     #20
  │
  ▼
store: new content is appended to observations (dedupe key)
       unchanged content only moves last_confirmed_at       #34
  │
  ▼
resolver ─ precedence, off the board, main line, confidence
  │        writes resolved_prices
  ▼
HTTP API ─ canonical prices + source health
```

The only boundary between a provider and the rest of the system is this
interface:

```go
type Provider interface {
    Source() canonical.SourceID
    Slices(ctx context.Context) ([]SliceKey, error)
    Poll(ctx context.Context, slice SliceKey) (Snapshot, error)
}
```

Nothing outside an adapter knows about a provider's schema. Nothing after the
canonicalization point branches on which provider sent the data.

## Canonical model and data ownership

We own event IDs, team identities, and the canonical price view. Providers
own only their raw observations.

- **Identity (#12).** `teams` plus `team_aliases` map each provider's spelling
  to one team. `events` plus `event_source_refs` map each provider's event ID
  to our event, matched on the teams plus start time.
- **Prices (#42).** Prices are decimal odds at full precision, and the API also
  shows them as American odds. The provider's raw value and
  its format are kept alongside for audit.
- **Lines (#29).** Lines are stored as written, e.g. `-3.5`, and parsed
  directly from the provider's value.
- **Markets (#10).** Moneyline, spread, and total, including alternate lines.

| Table | Role |
|---|---|
| `observations` | Append-only history of every accepted observation, with a unique content key |
| `quarantined_observations` | Raw records that failed sanity checks, with the reason |
| `resolved_prices` | The current view the API serves. It's always rebuilt from `observations`, never edited directly |
| `slice_health` | Health of each slice of each source: last success, last error, failure count, backoff, staleness threshold |

Because `resolved_prices` is derived, recovery after a crash or a rule change
is just a recompute.

## Rules

**Duplicates.** The content key is a hash of source, book, market, line,
side, price, status, and provider timestamp. Seeing the same observation
twice stores nothing new and only moves `last_confirmed_at` forward.

**Ordering (#31).** "Newer" always means the provider's timestamp. An
observation older than the current one goes into history but never replaces
the current price. When a source has no timestamp, the time we received it is
used, confidence is capped at medium, and a `no_source_timestamp` reason is
added (#32).

**Precedence (#15).** For each book, the direct feed wins while it's fresh.
When it goes stale, the aggregator is used. Every canonical price names the
source it came from.

**Freshness (#21, #28).** A successful poll of a slice counts as its
heartbeat. Each source has its own staleness threshold, because the aggregator
is expected to lag.

**Off the board (#17, #22, #25).**
- Any fresh source can mark a market off the board.
- A missing market counts as off the board only when its slice was fetched
  successfully and completely.
- A newer observation that isn't off the board puts the market back on.
- When unsure, we err toward off the board.

**Main line (#18).** We work out the main line ourselves: the line whose sides
are closest to a 50/50 implied probability. A provider's main-line flag is one
input, not the answer.

**Confidence (#26, #27).** The API shows a level (`high`, `medium`, or `low`)
plus the reasons behind it. The direct feed and the aggregator agree when
their implied probabilities are within a tolerance, compared on the same line.
The comparison allows for the aggregator's lag. Agreement raises confidence,
and staleness, disagreement, or a missing timestamp lowers it.

## Failure behavior

- **Slices are isolated (#33, #36).** A failed slice backs off with
  exponential backoff plus jitter. Every other slice keeps updating.
- **Each provider has one shared request budget.** On a 429, the budget
  honors `Retry-After`.
- **Partial success is normal.** Results from successful slices are applied
  straight away. Failed slices keep their last known state, and that state
  gets older and eventually shows as stale.
- **A provider can go down entirely.** The API keeps serving the best local
  answer: fall back to the other source, or serve the last known price marked
  stale. `/status` shows the source as degraded or down.

## Operability

- `/status` shows each source and slice: its state (healthy, degraded, or
  down), last success, last error, failure count, next retry, and freshness
  against its staleness threshold.
- Structured logs (`log/slog`) record one line per poll, carrying the source,
  slice, outcome, duration, and how many observations were accepted, left
  unchanged, or quarantined.
- Every canonical price carries its source, `observed_at`,
  `last_confirmed_at`, confidence level, and reasons, so a single API response
  shows how trustworthy each price is.

## Simulator (#38, #39)

The simulator runs in the same binary as the sync engine, with one HTTP
server per provider. Seeded random moves drift prices, move lines, and take
markets off the board and back. Scripted scenarios cover repeatable failures
such as an outage, aggregator lag, rate limiting, or malformed data. An admin
endpoint lets a reviewer trigger a scenario while the system is running.

## Proposed layout

```
cmd/dataflow/          main: starts the simulator, sync engine, and API
internal/canonical/    canonical types and odds/line conversion
internal/provider/     Provider interface
  aggregator/  draftkings/  fanduel/
internal/sanity/       observation sanity checks
internal/store/        SQLite (modernc.org/sqlite, pure Go, so no C compiler is needed)
internal/syncer/       slice scheduler, backoff, shared rate limiting
internal/resolve/      precedence, off the board, main line, confidence
internal/api/          HTTP read API and /status
internal/sim/          mock provider servers, scenarios, admin endpoint
```
