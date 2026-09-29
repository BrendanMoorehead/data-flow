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

Open **http://127.0.0.1:8080/** in a browser. The dashboard refreshes every
1.5s and shows:

- each source's health, including retrying and failing slices and their next attempt
- scenario buttons that break things for 30s (click an active one to end it early)
- every price with its status, source, confidence, and age

For example, click **draftkings-outage** and watch `draftkings_direct` go
`failing` while DraftKings prices switch to the aggregator.

The dashboard is one static HTML file embedded in the binary. It only reads
the API below, so everything it shows is also available with `curl`: see
[Example calls](#example-calls).

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

## How to verify

Each row is one behavior the brief asks about, the test that proves it, and
what to look for while it runs. Run one row's tests with
`go test ./... -run '<names>' -v`.

| Claim | Proven by | See it live |
|---|---|---|
| Three different schemas merge into one event per game | `TestProvidersWithDifferentIdentifiersMergeIntoOneEventPerGame`, `TestProviderWithHomeAndAwayReversedJoinsTheSameGameWithSidesFlipped` | The dashboard lists 6 games, each priced for DraftKings and FanDuel, although the providers spell teams differently (`Los Angeles Lakers` / `LA Lakers` / `Lakers`) and use different IDs |
| Seeing the same data twice changes nothing | `TestRecordObservationTwiceStoresOnceAndMovesConfirmation`, `TestRepeatedPollsDoNotDuplicateObservations` | Log lines show `inserted=0 confirmed=2` on repeat polls: stored once, only freshness moves |
| An older update never replaces a newer one | `TestLatestSourcePricesIgnoresOutOfOrderObservation` | Runs by default (DraftKings re-sends an old quote about 5% of the time). It's covered by the test, because nothing visibly changes when it works |
| Direct feed wins while fresh, then falls back to the aggregator | `TestSelectPreferredChoosesFreshDirectOverFreshAggregator`, `TestSelectPreferredFallsBackToAggregatorWhenDirectIsStale` | Click **draftkings-outage**. Within ~12s, "What just happened" shows DraftKings prices switching to the aggregator, then back when it ends |
| A slow provider doesn't hold up the others | `TestHangingSliceDoesNotStopOtherSlicesFromPolling` | Click **draftkings-slow**. DraftKings goes `retrying` while FanDuel and the aggregator stay `healthy` |
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

# End it early, and the prices switch back to draftkings_direct
curl -s -X DELETE localhost:9100/scenarios/draftkings-outage
```

The other scenarios are `draftkings-slow`, `aggregator-rate-limit`, and
`aggregator-lag`.

## AI & Tools

### Why `.claude/skills/` is committed

This repo includes the Claude Code skills I used while building it. They are
part of the submission on purpose:

- **They show how I steered the AI.** A skill holds the rules I made the
  assistant follow for this codebase, such as how a provider adapter must be
  shaped and where canonicalization happens. Reading them shows which decisions
  I made and which work I delegated.
