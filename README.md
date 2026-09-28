# data-flow

Sportsbook odds sync engine (Pikkit BE-01 take-home).

Simulated sportsbook feeds with different schemas are synced into one
canonical odds view. The system overview and failure semantics are in
[docs/design.md](docs/design.md). The reasoning behind each choice is in
[docs/decisions.md](docs/decisions.md).

## Run it

You need Go 1.24 or newer. There is nothing else to install, because the
SQLite driver is pure Go.

```sh
git clone https://github.com/BrendanMoorehead/data-flow.git
cd data-flow
go run ./cmd/dataflow
```

That one command starts the simulated providers, the sync engine, and the
read API on `http://127.0.0.1:8080`. State lives in `data-flow.db`. Delete
that file to start fresh.

To run the tests:

```sh
go test ./...
```

## Example calls

```sh
# Canonical events, merged across providers
curl -s localhost:8080/events

# Resolved odds for one event, with source and freshness on every price
curl -s localhost:8080/events/1/odds

# Sync health for each source and each slice
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
