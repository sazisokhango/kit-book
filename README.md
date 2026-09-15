# kitbook

Offline CLI equipment-booking tool for Ridgeline Mountain Rescue — checkout, checkin, status,
and history for the equipment store, replacing a whiteboard.

Built against the SDLC artefacts in the sibling `pg-ai-sdlc/` framework repo:

- Spec: `../pg-ai-sdlc/01-sow/01-sow-brief.md`
- Architecture (ADRs, diagrams): `../pg-ai-sdlc/04-architecture/`
- Unit specs: `../pg-ai-sdlc/05-spec/units/`

## Stack

- Go 1.23+ (ADR-001)
- Embedded SQLite via `modernc.org/sqlite`, a pure-Go driver — no CGO (ADR-002)
- `spf13/cobra` for the CLI command tree (ADR-003)

## Build

```sh
go build -o kitbook ./cmd/kitbook
./kitbook version
./kitbook doctor   # health check: verifies core wiring + DB schema/connectivity
```

## Status

Sprint Zero (infrastructure) complete: compiles, tests green, `version`/`doctor` hello-world
commands wired end-to-end through CLI → core → SQLite store. Sprint 1 (U1 data model, U7
service-due integration, U3 checkout, U4 checkin, U5 status board) is next — see
`../pg-ai-sdlc/05-spec/dependency-graph.md`.
