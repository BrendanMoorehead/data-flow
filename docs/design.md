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
| `draftkings_direct` | Direct feed, one slice per game | American odds as strings, a timestamp on each offer, abbreviated team names | `SUSPENDED` on the whole offer |
| `fanduel_direct` | Direct feed, one slice per league (#52) | American odds as integers, nickname-only teams, epoch-second start times, **no timestamps** (#32), and a price is occasionally missing (#53) | `SUSPENDED` on each side separately |

Every source is polled as a **full refresh** (#33). Each adapter splits its
provider into **slices** that match how that provider's site divides its data,
for example by league page or by game (#35).

## Lifecycle of a record

```
simulator (local HTTP mock servers, one per provider)          #37
  │
  ▼
poller: worker pool per provider, backoff per slice           #36, #43
  │     one request budget per provider
  ▼
adapter.Poll(slice) ─ fetch + parse this provider's schema
  │
  ▼
[] Observation ◀── step 1, OUR UNITS: decimal odds, our book / market /
  │                side / status labels. The event is still described in
  │                the provider's terms (its ID, its team spellings).
  ▼
sanity checks ─ fails → quarantined_observations               #54, #55
  │
  ▼
identity ◀────── step 2, OUR IDENTITY: team aliases + start time
  │               → our event ID. Unknown team → quarantined.  #67, #68
  ▼
observations ─ new content appended (content key)
  │            repeated content only moves last_confirmed_at   #34
  ▼
resolver ─ precedence, off the board, confidence → resolved_prices
  │
  ▼
HTTP API and dashboard
```

[normalization.md](normalization.md) walks one real game through these
steps, showing each provider's payload and the rows it becomes.

Data becomes ours in **two named steps**. The adapter translates units, and
the identity step assigns our event. They're separate because identity needs
shared state (the alias table and existing event links), while adapters stay
pure parsing code that needs no database. Sanity checks run **between** them
on purpose: identity can *create* events and links, so bad data is
quarantined before anything permanent is written.

The only boundary between a provider and the rest of the system is this
interface:

```go
type Provider interface {
    Source() canonical.SourceID
    Slices(ctx context.Context) ([]SliceKey, error)
    Poll(ctx context.Context, slice SliceKey) (Snapshot, error)
}
```

Nothing outside an adapter knows about a provider's schema. Shared code
never branches on a provider's name. It only reacts to what the data
declares: a source's role (`direct` or `aggregator`), whether missing
markets mean off the board (`MissingMeansOffBoard`), whether it sent a
timestamp (`HasSourceTimestamp`), and how precisely it rounds prices
(`PriceDecimals`).

## Canonical model and data ownership

We own event IDs, team identities, and the canonical price view. Providers
own only their raw observations.

- **Identity (#12, #67, #68).** Teams and each provider's spelling of them
  live in data files, `fixtures/teams.csv` and `fixtures/team_aliases.csv`
  (one alias per row). They're checked and loaded into `teams` and
  `team_aliases` at startup; a duplicate spelling or an alias for an unknown
  team stops startup. Adding a provider's names means adding rows.
  `event_source_refs` maps each provider's event ID to our event. A new one is
  matched on the **unordered** pair of teams plus a start time within ±30
  minutes. If a provider lists home and away the other way round, the link is
  flagged `sides_swapped`, and that provider's home and away are flipped when
  its prices are ingested.
- **Prices (#42).** Prices are decimal odds at full precision, and the API also
  shows them as American odds. The provider's raw value and
  its format are kept alongside for audit.
- **Markets.** Moneyline only. Spreads, totals, and alternate lines were
  designed but not built, in favor of failure handling. Adding them means putting the line in `PriceKey` and working out
  each game's main line in the resolver.

| Table | Role |
|---|---|
| `observations` | Append-only history of every accepted observation, with a unique content key |
| `quarantined_observations` | Raw records that failed sanity checks, with the reason |
| `resolved_prices` | The current view the API serves. It's always rebuilt from `observations`, never edited directly |
| `slice_health` | Health of each slice of each source: last success, last error, failure count, backoff, staleness threshold |

Because `resolved_prices` is derived, recovery after a crash or a rule change
is just a recompute.

## Rules

**Sanity checks and quarantine (#20, #54, #55).** Before storage, any record
the adapter can't parse goes to `quarantined_observations` with its raw JSON
and a reason. So does any market whose two open sides imply a book margin
outside 1.00–1.15. A single open side passes, because one side can
legitimately be off the board.

**Duplicates.** The content key is a hash of source, book, market, side,
status, price, and provider timestamp. Seeing the same observation
twice stores nothing new and only moves `last_confirmed_at` forward.

**Ordering (#31).** "Newer" always means the provider's timestamp. An
observation older than the current one goes into history but never replaces
the current price. When a source has no timestamp, the time we received it is
used, confidence is capped at medium, and a `no_source_timestamp` reason is
added (#32).

**Precedence (#15).** For each book, the direct feed's price wins while it's
fresh. When it goes stale, the aggregator's price is used. Every price names
the source it came from.

**Freshness (#21, #28).** A successful poll of a slice counts as its
heartbeat. Each source has its own staleness threshold, because the aggregator
is expected to lag.

**Off the board (#17, #22, #25, #56–59).**
- A market's status comes from its **most recent fresh observation**. Any
  fresh source can take a market off the board, and a newer open observation
  puts it back on. On a timestamp tie, off the board wins.
- The aggregator shows off the board by leaving a market out. After a
  **complete** snapshot, the syncer compares the slice with that source's
  last known markets. Each missing one gets an `off_board` record at poll
  time. If that source's latest record is already off the board, it's only
  confirmed. This keeps the time it went off fixed, and stops the market
  flipping back and forth while a direct feed disagrees.
- A market that is off the board still carries its **last known price**. The
  `status` field is the main indicator.

**Confidence (#26, #27, #60–62).** Every price has a level plus the reasons
behind it. The served price is compared with the best other fresh source for
the same book, market, and side:

| Level | When |
|---|---|
| `verified` | The two sources match exactly, compared at the coarser source's precision (`PriceDecimals`: 2 for the aggregator; exact for sources sending American odds) |
| `high` | Their implied probabilities are within 1.5 percentage points |
| `medium` | Only one fresh source, or the served source has no timestamps (capped) |
| `low` | The sources disagree, or every source is stale |

The reasons come from a fixed list: `direct_fresh`, `direct_stale`,
`sources_match`, `sources_agree`, `sources_disagree`, `no_second_source`,
`no_source_timestamp`, `all_sources_stale`. The comparison uses the direct
feed's *current* price, so while the aggregator lags, its prices honestly
show `sources_disagree`.

## Failure behavior

- **A worker pool per provider (#43).** A dispatcher hands due slices to a
  fixed pool of workers: 3 for DraftKings (one slice per game), 1 each for
  FanDuel and the aggregator (one slice per league). A
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
  confirmed, inferred off the board, and quarantined. A failure's line includes the error and the next
  attempt time.
- Every price carries its status, source, `observed_at`, `last_confirmed_at`,
  freshness against its source's threshold, and confidence with reasons.
- `/status` shows how many observations each source has had quarantined.
- A read-only dashboard at `/` (#64) shows all of this live. It's one HTML file
  embedded with `go:embed`, polling the same endpoints. Its scenario buttons
  call the simulator's admin API, which allows requests from the dashboard's
  address (#65).

## Simulator (#38, #39, #49–51)

The simulator runs in the same binary. It has one HTTP server per provider
and an admin server on its own port.

- **World.** Seeded random moves drift prices. Quote history is kept, so a
  provider can serve an old or lagged price.
- **Background problems, on by default (#49).** About 5% of DraftKings
  requests return a 500. About 5% of responses carry the previous, older
  quote (out of order). About 3% repeat an outcome (duplicate records).
  About 3% of FanDuel prices are missing. Run with `-calm` to turn them off.
- **Markets go off the board (#63).** Now and then, a book's market for one
  game is suspended for 10–20s, sometimes on one side only. Each provider
  shows this in its own way (see Sources). This is normal market behavior,
  so it also happens with `-calm`.
- **Scripted scenarios (#50).** Triggered from the admin API:

| Scenario | Effect |
|---|---|
| `draftkings-outage` | Every DraftKings request returns 503 |
| `draftkings-slow` | Responses take 3s, longer than the 2s timeout |
| `aggregator-rate-limit` | One request per 5s is allowed; the rest get 429 with `Retry-After` |
| `aggregator-lag` | The aggregator serves prices from 20s ago |

## Layout

```
cmd/dataflow/          main: wires the simulator, sync engine, and API into one process
fixtures/              teams.csv and team_aliases.csv, embedded into the binary
internal/canonical/    our data model: observations, keys, statuses, confidence
internal/odds/         American and decimal odds conversion
internal/source/       per-source config: role, intervals, timeouts, budget, precision
internal/provider/     the Provider interface and shared snapshot/HTTP helpers
  aggregator/  draftkings/  fanduel/     one adapter per provider schema
internal/sanity/       margin checks before anything is stored
internal/identity/     team aliases and event matching (loads fixtures/)
internal/store/        SQLite (modernc.org/sqlite, pure Go, no C compiler needed)
internal/syncer/       worker pools, slice schedule, catalog refresh, ingest
internal/backoff/      exponential backoff with jitter
internal/ratelimit/    token-bucket budget per provider, honoring Retry-After
internal/resolve/      precedence, off the board, confidence → resolved_prices
internal/api/          read API, /status, embedded dashboard
internal/httpserve/    JSON response helpers
internal/sim/          mock providers, background problems, scenarios, admin API
```
