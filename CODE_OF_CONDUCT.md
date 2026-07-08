# Coding Standards

This document defines coding standards for contributors to this repo. It is
not a community code of conduct — it's a map to how we write code here, and to
the authoritative sources for specific practices (Claude Code skills,
architecture docs) rather than a restatement of them.

## Language & tooling

- Go 1.26, module `taxi-platform`.
- `gofmt`/`goimports` are non-negotiable — run them before every commit. No
  manual style debates on anything gofmt settles.
- `golangci-lint` is required and is CI-blocking policy: a lint failure is
  treated like a build failure. (Note: CI isn't wired up yet — this is the
  target the moment it is, not a future nice-to-have.)

## Architecture conventions

Core values are DDD, event-driven architecture, and strong type safety — see
`taxi-platform/02 Architecture Principles.md` for the full picture. In code,
this means:

- **Command → domain method → event → handler** as the standard flow through
  the backend.
- Domain packages own their own state machine and events, one vertical slice
  per feature under `internal/<feature>/` (e.g. `internal/trip/`) rather than
  a shared generic layer.
- Domain/service methods are named after the business event or command they
  represent (`MarkNoShow`, `AdjustOccupancy`), never generic CRUD verbs
  (`Update`, `Set`).

## Comments & documentation

Default to no comments. Well-named identifiers should already say what code
does. Only add a comment when it captures something a reader couldn't get
from the code itself: a hidden constraint, a subtle invariant, a workaround
for a specific bug. No multi-paragraph docstrings, and no comments that just
restate the following line in English.

## Error handling

Fail fast. Don't write defensive checks, fallbacks, or validation for
scenarios that can't happen — trust internal invariants and let bugs surface
as errors/panics rather than masking them. Validate only at true system
boundaries (HTTP input, external API responses, etc.).

Use typed errors and assert them with `errors.As`, never by matching on
`err.Error()` strings.

## Testing

TDD is required for domain and service logic under `internal/` and `cmd/`.
The full discipline — red-green-refactor loop, white-box test packages,
scenario-named tests, hand-rolled fakes only at I/O boundaries — is codified
in `.claude/skills/tdd/SKILL.md`. Read that before writing new domain/service
code; this doc won't restate it.

## Docs stay in sync

Any code change under `internal/` or `cmd/` that adds, changes, or removes a
capability must be reflected in the `taxi-platform/` Obsidian vault. This is
codified in `.claude/skills/vault-docs/SKILL.md` and enforced by
`.claude/hooks/doc-review-stop.sh`, which blocks finishing a turn if Go
source changed without a matching doc update.
