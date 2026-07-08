# CLAUDE.md

Operating guide for agents working in this repo. For coding style, comment,
and error-handling rules, see `CODE_OF_CONDUCT.md` — read it before writing
code; this file won't restate it.

## What this repo is

A Go backend for a South African minibus-taxi platform, built DDD-style and event-driven. `taxi-platform/` is a separate
Obsidian vault of living docs — start at `taxi-platform/00 Home.md` and
`taxi-platform/01 Overview.md` for the product/architecture picture before
diving into code.

## Repo map

- `cmd/api` — the HTTP process: Postgres pool, in-process event bus, outbox
  dispatcher wiring. Only `/healthz` exists today.
- `cmd/mockclient` — load-test traffic simulator; currently just pings
  `/healthz` as a placeholder for real trip endpoints.
- `internal/trip` — the trip domain: state machine, service layer, events.
- `internal/platform/eventbus` — in-process pub/sub, designed to be swapped
  for NATS/Kafka later.
- `internal/platform/postgres` — connection pool and the outbox dispatcher.
- `migrations/` — SQL schema (trips, outbox_events).
- `taxi-platform/` — docs vault, not code. `Features/` mirrors domain
  packages; `Planning/` holds the roadmap, ideas, and bugs.

## Required workflows

Two Claude Code skills already govern changes here — don't skip them or work
around their triggers:

- **TDD** (`.claude/skills/tdd/SKILL.md`) — mandatory red-green-refactor for
  any behavior change to `internal/`/`cmd/` Go logic: failing test first,
  minimal code to pass, then refactor.
- **Vault docs** (`.claude/skills/vault-docs/SKILL.md`) — any capability
  change under `internal/`/`cmd/` must be reflected in the matching
  `taxi-platform/Features/*.md` note. This isn't just a suggestion: a `Stop`
  hook (`.claude/hooks/doc-review-stop.sh`) blocks finishing a turn if `.go`
  files changed under `internal`/`cmd` with no matching vault update.

## Verifying changes

There's no CI, Makefile, or linter config yet. Before calling a change done,
run:

```
go build ./...
go test ./...
```

`gofmt`/`goimports` are expected per `CODE_OF_CONDUCT.md`.

## Before proposing new work

Check `taxi-platform/Planning/Roadmap.md` for the agreed build sequence and
open items first, so you don't propose work that's already sequenced or
duplicate an idea/bug that's already tracked in `Planning/`.
