# data-flow

Sportsbook odds sync engine (Pikkit BE-01 take-home).

Simulated sportsbook feeds with different schemas are synced into one
canonical odds view. The system overview and failure semantics are in
[docs/design.md](docs/design.md). The reasoning behind each choice is in
[docs/decisions.md](docs/decisions.md).

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

With it running, open a second terminal and use the calls in
[Example calls](#example-calls) below.

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
