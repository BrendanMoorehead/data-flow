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

- **A worker pool per provider (#43).** A dispatcher hands due slices to a
  fixed pool of workers: 3 for each direct feed, 1 for the aggregator. A
  slow slice ties up only one worker, and the per-source request timeout
  (#45) caps how long it can do so. The trade-off: if every worker hangs at
  once, that provider's other slices wait up to one timeout.
- **Retries follow backoff, never a tight loop (#46).** After a failure, the
  slice's next attempt is its poll interval doubled for each consecutive
  failure, capped at 60s. Half of that delay is fixed and the other half is
  random (jitter), so retries from many slices don't line up. One success
  resets the count.
- **Each provider has one shared request budget (#47).** A token bucket
  limits the rate of requests. A 429 pauses the *whole* provider until its
  `Retry-After`, because rate limits apply to the account, not to one slice.
  This lives in an HTTP transport, so adapters don't know it exists.
- **The catalog is refreshed periodically (#44).** If fetching the list of
  slices fails, the last known list keeps being polled. A broken event list
  doesn't stop healthy games from updating.
- **Partial success is normal.** Results from successful slices are applied
  straight away. A failed slice keeps its last known prices, which grow older
  and eventually show as stale. An aggregator poll counts as successful only
  if every page was fetched.
- **A provider can go down entirely.** The API keeps serving the best local
  answer. Once the DraftKings direct feed goes stale, DraftKings prices fall
  back to the aggregator's copy. When the feed recovers, they switch back.

## Operability

- `/status` shows each source and each slice: its state, last success, time
  since that success, last error, consecutive failures, and `next_attempt_at`.
  - **States (#48):** `healthy`; `retrying` (1–2 consecutive failures);
    `failing` (3 or more); `stale` (no recent success).
  - A source reports its worst slice's state.
  - The `catalog` slice is judged against its refresh interval, not the
    price staleness threshold.
- Structured logs (`log/slog`) record one line per poll. Each line has the
  source, slice, and duration, plus counts of observations inserted, only
  confirmed, and rejected. A failure's line includes the error and the next
  attempt time.
- Every price carries its source, `observed_at`, `last_confirmed_at`, and
  freshness against its source's threshold.

## Simulator (#38, #39, #49–51)

The simulator runs in the same binary. It has one HTTP server per provider
and an admin server on its own port.

- **World.** Seeded random moves drift prices. Quote history is kept, so a
  provider can serve an old or lagged price.
- **Background problems, on by default (#49).** About 5% of DraftKings
  requests return a 500. About 5% of responses carry the previous, older
  quote (out of order). About 3% repeat an outcome (duplicate records). Run
  with `-calm` to turn them off.
- **Scripted scenarios (#50).** Triggered from the admin API:

| Scenario | Effect |
|---|---|
| `draftkings-outage` | Every DraftKings request returns 503 |
| `draftkings-slow` | Responses take 3s, longer than the 2s timeout |
| `aggregator-rate-limit` | One request per 5s is allowed; the rest get 429 with `Retry-After` |
| `aggregator-lag` | The aggregator serves prices from 20s ago |

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
