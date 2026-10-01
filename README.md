# data-flow

Sportsbook odds sync engine (Pikkit BE-01 take-home).

Simulated sportsbook feeds with different schemas are synced into one
canonical odds view.

- [docs/normalization.md](docs/normalization.md) follows one game from all
  three provider schemas into our schema, using real captured data.
- [docs/design.md](docs/design.md) covers the system overview and failure
  semantics.
- [docs/decisions.md](docs/decisions.md) records the reasoning behind each
  choice.

## Run it

### 1. Install Go

You need **Go 1.24 or newer** ([download](https://go.dev/dl/)). Check with:

```sh
go version
```

There is nothing else to install: no Docker, no database server, and no
C compiler, because the SQLite driver is pure Go. `curl` is enough to try the
API.

### 2. Clone and start

```sh
git clone https://github.com/BrendanMoorehead/data-flow.git
cd data-flow
go run ./cmd/dataflow
```

The first run downloads the Go dependencies, which takes a few seconds. After
that, one process starts everything:

| What | Address |
|---|---|
| **Dashboard** | **`http://127.0.0.1:8080/`** |
| Read API | `http://127.0.0.1:8080` |
| Simulator scenario admin | `http://127.0.0.1:9100` |
| Simulated aggregator | `http://127.0.0.1:9101` |
| Simulated DraftKings feed | `http://127.0.0.1:9102` |
| Simulated FanDuel feed | `http://127.0.0.1:9103` |

The terminal prints one structured log line per poll. After a few seconds you
should see `msg="poll succeeded"` lines for all three sources. Occasional
`poll failed` and `observation quarantined` lines are the simulator's
background problems, which is expected. Press **Ctrl+C** to stop.

### 3. Try it

Open **http://127.0.0.1:8080/** in a browser. See
[Reading the dashboard](#reading-the-dashboard) for what each section means.
Everything the dashboard shows comes from the read API, so it's also
available with `curl`: see [Example calls](#example-calls).

### Options

| Flag | Default | Purpose |
|---|---|---|
| `-calm` | off | Turn off background problems (random 500s, out-of-order updates, duplicates, missing prices) |
| `-seed` | `42` | Seed for simulated price movement and problems |
| `-db` | `data-flow.db` | SQLite file holding all state |
| `-addr` | `127.0.0.1:8080` | Read API address |
| `-sim-admin-addr` | `127.0.0.1:9100` | Scenario admin address |
| `-aggregator-addr`, `-draftkings-addr`, `-fanduel-addr` | `9101`, `9102`, `9103` | Simulated provider addresses |

For example, `go run ./cmd/dataflow -calm -db /tmp/calm.db`.

### Start fresh

All state is in `data-flow.db` (plus `-wal` and `-shm` files beside it).
Stopping and restarting keeps it: events and history survive and prices are
rebuilt. To reset:

```sh
rm -f data-flow.db data-flow.db-*
```

### Run the tests

```sh
go test ./...
go test -race ./...   # also checks for concurrency bugs
```

### Troubleshooting

- **`listen for ... address already in use`**: another process holds that
  port. Stop it, or move ours with the flags above, e.g. `-addr 127.0.0.1:8081`.
- **`apply schema` or `no such column` errors on start**: the database was
  created by an older version. Delete it (see [Start fresh](#start-fresh)).

## Reading the dashboard

The dashboard (`http://127.0.0.1:8080/`) refreshes every 1.5s. Each section
answers one question, from top to bottom:

**1. Sync health: is each source keeping up right now?** One line per
source.

| State | Meaning |
|---|---|
| `healthy` | Every slice polled successfully within the source's staleness threshold |
| `retrying` | A slice failed once or twice and is waiting for its next attempt; usually a blip |
| `failing` | A slice failed 3 or more times in a row; the line shows how long since the last success and when the next retry is |
| `stale` | No recent success, but no current error either |

The quarantine count is the number of bad records rejected from that source.
It rises steadily for FanDuel, which sometimes sends a runner with no price.

**2. Break something.** Each button runs a simulator scenario for 30s. Click
it again while it's active to end it early.

**3. What just happened: what has the sync engine noticed?** A feed of
meaningful changes since the page opened: sources failing or recovering,
scenarios starting or ending, a book's prices switching source, and markets
going off the board or coming back. Price moves and one-off retries are left
out on purpose, so each line is worth reading.

**4. Where each price comes from.** One row per game and one cell per book.
Each cell shows the source being served, the away / home prices, and a color
for how far to trust them. Click a game for the full detail behind each price:
decimal odds, confidence reasons, and seconds since last confirmed.

| Color | Label | Meaning |
|---|---|---|
| green | `confirmed` | A second fresh source matches or closely agrees |
| yellow | `one source` | Nothing fresh to compare against, e.g. DraftKings falling back to the aggregator during an outage |
| yellow | `no timestamps` | A second source agrees, but this feed never says when its prices changed, so confidence is capped |
| red | `doubtful` | The sources disagree, or the data is stale |
| grey | `off board` | The market is pulled; the price shown is the last one seen |

**What normal looks like:** all three sources `healthy`; DraftKings cells
green (its direct feed is confirmed by the aggregator); FanDuel cells yellow
`no timestamps`; the occasional market going grey and coming back, which is
the simulator suspending markets; and a brief red `doubtful` right after a
price moves, until the other source's next poll catches up a few seconds
later.

**A two-minute tour:**
1. Click **draftkings-outage**. Within a few seconds, `draftkings_direct` goes
   `retrying`, then `failing`.
2. After about 12s, the DraftKings cells switch to `aggregator` and turn
   yellow `one source`. The feed logs each switch. The system is still
   serving the best price it has, and it tells you it's less sure.
3. Click the button again to end it. As each slice's next retry succeeds,
   the cells switch back to `draftkings_direct` and turn green. Backoff can
   make this take up to a minute.

## How to verify

Each row is one behavior the brief asks about, the test that proves it, and
what to look for while it runs. Run one row's tests with
`go test ./... -run '<names>' -v`.

| Claim | Proven by | See it live |
|---|---|---|
| Three different schemas merge into one event per game | `TestProvidersWithDifferentIdentifiersMergeIntoOneEventPerGame`, `TestProviderWithHomeAndAwayReversedJoinsTheSameGameWithSidesFlipped` | The dashboard lists 6 games, each priced for DraftKings and FanDuel, although the providers spell teams differently (`Los Angeles Lakers` / `LA Lakers` / `Lakers`) and use different IDs |
| Seeing the same data twice changes nothing | `TestRecordObservationTwiceStoresOnceAndMovesConfirmation`, `TestRepeatedPollsDoNotDuplicateObservations` | Log lines show `inserted=0 confirmed=2` on repeat polls: stored once, only freshness moves |
| An older update never replaces a newer one | `TestLatestSourcePricesIgnoresOutOfOrderObservation` | Runs by default (DraftKings re-sends an old quote about 5% of the time). It's covered by the test, because nothing visibly changes when it works |
| Direct feed wins while fresh, then falls back to the aggregator | `TestSelectPreferredChoosesFreshDirectOverFreshAggregator`, `TestSelectPreferredFallsBackToAggregatorWhenDirectIsStale` | Click **draftkings-outage**. Within ~12s, "What just happened" shows DraftKings prices switching to the aggregator, then back once its next retry succeeds (backoff can make that take up to a minute) |
| A slow provider doesn't hold up the others | `TestHangingSliceDoesNotStopOtherSlicesFromPolling` | Click **draftkings-slow**. DraftKings goes `retrying`, then `failing`, while FanDuel and the aggregator stay `healthy` |
| Retries back off with jitter, never in a tight loop | `TestDelayDoublesWithEachConsecutiveFailure`, `TestDelayNeverExceedsCeiling`, `TestFinishAfterFailureDelaysAndCountsFailures` | During an outage, the next-retry times in "Sync health" move further out |
| Rate limits pause the whole provider | `TestRateLimitedResponsePausesTheWholeBudget`, `TestAggregatorRateLimitAllowsOneRequestThenSendsRetryAfter` | Click **aggregator-rate-limit**. Only a couple of `429` lines appear in the logs, not one per request |
| Bad or incomplete records are quarantined, not stored | `TestRunnerWithoutPriceIsRejected`, `TestCheckMarginsRejectsBothSidesOfAnImpossibleMarket` | FanDuel's quarantine count in "Sync health" rises (about 3% of its prices arrive without one) |
| Off the board is inferred safely and reverses | `TestMarketMissingFromCompleteSnapshotGoesOffTheBoard`, `TestIncompleteSnapshotsNeverInferOffBoard`, `TestStillMissingMarketIsConfirmedNotReinferred`, `TestNewerOpenObservationPutsMarketBackOnTheBoard` | "What just happened" shows markets going off the board and coming back as the simulator suspends them |
| Confidence says how trustworthy each price is | `TestConfidenceIs*`, `TestMissingSourceTimestampCapsConfidenceAtMedium` | DraftKings is mostly green (confirmed by the aggregator). FanDuel stays yellow because its feed has no timestamps |
| Health is visible at a glance | `TestSliceStateEscalatesWithConsecutiveFailures`, `TestCatalogIsJudgedAgainstItsRefreshInterval` | "Sync health" at the top of the dashboard, or `curl -s localhost:8080/status` |

## Example calls

```sh
# Canonical events, merged across providers
curl -s localhost:8080/events

# Resolved odds for one event: status, source, freshness, and confidence on every price
curl -s localhost:8080/events/1/odds

# Sync health for each source and each slice, plus quarantine counts
curl -s localhost:8080/status
```

## Break things on purpose

The simulator produces mild problems by default: intermittent 500s,
out-of-order updates, and duplicate records. Run with `-calm` to turn them
off. Stronger failures are triggered from the simulator's admin API on port
9100:

```sh
# List scenarios and whether each is active
curl -s localhost:9100/scenarios

# Take the DraftKings direct feed down for 40 seconds
curl -s -X POST "localhost:9100/scenarios/draftkings-outage?for=40s"

# Watch its slices go retrying → failing, with next_attempt_at backing off
curl -s localhost:8080/status

# After ~12s, DraftKings prices switch to "source": "aggregator"
curl -s localhost:8080/events/1/odds

# End it early. Prices switch back to draftkings_direct once each slice's
# next retry succeeds (after several failures, backoff waits up to 60s)
curl -s -X DELETE localhost:9100/scenarios/draftkings-outage
```

The other scenarios are `draftkings-slow`, `aggregator-rate-limit`, and
`aggregator-lag`.

## At higher scale: what I'd change first

In order, starting with what would break first:

1. **Rebuild resolved prices incrementally.** The resolver currently rebuilds
   every price from the full `observations` history once a second. That's
   fine for a demo (about 16k rows an hour) but grows without limit. I'd
   resolve only the price keys an ingest actually touched, re-check freshness
   on a timer, and move old history to cold storage.
2. **Split polling from serving, and coordinate pollers.** Everything runs in
   one process with an in-memory schedule. At scale, poller workers would
   take **leases** on slices so two instances never poll the same one. The
   per-provider request budget would move to a shared store (for example a
   Redis token bucket), because a rate limit belongs to the account, not to
   one process.
3. **Move to Postgres.** SQLite has a single writer. I'd partition
   `observations` by time and serve the API from a read replica.
4. **Accept push feeds.** Many real feeds stream. Only the scheduler assumes
   polling; the ingest pipeline takes snapshots however they arrive, so a
   streaming adapter would call it directly.
5. **Turn identity into an operated workflow.** Team aliases are a reviewed CSV
   today. At scale, unmatched names would go to a review queue with suggested
   matches, and markets would need identity too (spread and total lines,
   player props).
6. **Metrics and alerts, not just logs and `/status`.** Poll latency,
   failure rate, and staleness per source, with alerts on `failing` and
   `stale`.

## AI & Tools

**Tools used**
- **Claude Code** (Anthropic's Claude, in VS Code) for design discussion,
  implementation, tests, and documentation.
- **gstack's headless browser**, driven through Claude Code, to check that the
  dashboard renders, has no console errors, and that its scenario buttons
  actually trigger failover.
- The standard Go tools (`gofmt`, `go vet`, `go test -race`) and the GitHub CLI.

**How I kept the decisions mine.** Two Claude Code skills are committed in
`.claude/skills/` so the setup is visible:
- `clean-code` holds my coding standards.
- `decision-checkpoints` makes the assistant stop at every design choice,
  present options and a recommendation, and wait for my call.

I logged every call along with what the AI proposed, and
[docs/decisions.md](docs/decisions.md) keeps the ones that shaped the system.

**Where I materially changed the AI's output**
- **I rejected its polling design (#33).** It proposed polling the direct feeds
  for "changes since my last timestamp." Sportsbooks generally don't offer
  that; scraping a book is a full poll. I changed it to full refreshes, split
  into small per-game or per-league slices so a failure only affects its own
  slice. That decision shaped the syncer: worker pools, per-slice backoff,
  and "missing means off the board only after a complete fetch."
- **I redesigned the provider model (#11, #14).** It framed each book as having
  one source. I made each book reported by both a direct feed and the
  aggregator, which creates the real conflicts the confidence and
  precedence rules exist to handle.
- **I rejected its first dashboard (#66).** It showed everything with equal
  weight. I had it rebuilt around three questions, and added the "How to
  verify" table above.

**Where a test caught the AI being wrong (#42).** It told me that 2-decimal odds
convert back to the true American price "except for heavy favorites," and I
chose the storage format on that basis. A unit test showed `−150 → 1.67 → −149`.
Measuring it found that 90% of favorite prices fail the round trip. We switched
to full-precision decimal storage and compare sources at the coarser source's
precision.
