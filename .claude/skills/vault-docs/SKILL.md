---
name: vault-docs
description: Use this skill whenever code under internal/ or cmd/ changes in a way that adds, changes, or removes a capability, or when asked to document a feature, brainstorm/log an idea, log a bug, or update the roadmap for the taxi-platform project. Keeps the taxi-platform/ Obsidian vault in sync with the code. Does not apply to pure code review or non-taxi-platform work.
---

# Vault documentation composer for taxi-platform

`taxi-platform/` is an Obsidian vault serving as living documentation for this repo. It has two areas:

- `taxi-platform/Features/` — one note per feature, kept accurate to current code.
- `taxi-platform/Planning/` — brainstormed ideas, planned work, and bugs, kept separate from stable feature docs.

Plus four root notes (`00 Home.md`, `01 Overview.md`, `02 Architecture Principles.md`, `03 Learnings and Principles.md`) covering the project as a whole.

## Feature notes

One feature ≈ one domain package, matching the existing mapping: `internal/trip` → `Features/Trip State Machine.md`. When code changes touch a feature:

1. Find the matching note in `Features/`. If none exists for a genuinely new feature, create one using the template below.
2. Update `## Capabilities` to match current code exactly — add what's new, remove or reword what no longer applies. Don't let this drift; a stale capability is worse than a missing one.
3. Update `## Implementation` file pointers and `## Status`.
4. Reword `## TLDR` only if the one/two-sentence summary of what the feature does has actually changed.

Template for a new `Features/<Name>.md`:

```markdown
# <Feature Name>

## TLDR
One or two sentences: what this feature does and why it exists.

## Capabilities
- Current, code-accurate bullet list of what's supported today.

## Implementation
- Pointers to source, e.g. `internal/trip/domain.go` — state machine

## Status
- e.g. "Shipped", "In progress (roadmap step 3)", "Planned"

## Related
- [[wikilinks]] to other Feature or Planning notes
```

## Planning notes

Brainstormed ideas, planned work, and bugs are **not** feature docs — they live in `Planning/` and don't get merged into a feature note's Capabilities until the work actually ships.

- **Idea** → `Planning/Ideas/<slug>.md`
- **Bug** → `Planning/Bugs/<slug>.md`
- **Planned work** → an entry in `Planning/Roadmap.md`'s "Open items", linking out to an Ideas/Bugs note rather than duplicating its content inline

Idea template:

```markdown
# <Idea Title>
Status: Raised
Date: YYYY-MM-DD

## TLDR
What the idea is, in one or two sentences.

## Details
Rationale, open questions, rough shape of the work.

## Related
- [[Feature note]] if it extends/touches an existing feature
```

Bug template:

```markdown
# <Bug Title>
Status: Open | In Progress | Fixed
Date found: YYYY-MM-DD

## TLDR
One-sentence symptom description.

## Details
Repro steps, root cause if known, affected code paths.

## Related
- [[Feature note]] for the affected feature
```

When a bug is fixed or an idea is implemented, update its `Status` line rather than deleting the note — the vault is a history, not just a current-state snapshot.

## Linking & vault hygiene

- Use bare `[[Note Name]]` wikilinks — no folder path, no number prefix. Obsidian resolves links by basename regardless of which folder the note lives in.
- Root notes (`00`–`03`) keep their number prefix in both filename and link text, since that prefix drives their sequencing in Obsidian's file explorer. `Features/` and `Planning/` notes never have a number prefix — folder placement already groups them.
- Whenever you add or move a note, update `00 Home.md`'s Map section and the `## Related` section of any note that should now point to it.

## What NOT to do

- Don't invent capabilities, bugs, or ideas that aren't grounded in the actual code or the current conversation.
- Don't leave a capability documented after the code that implements it is removed — delete or reword the bullet, don't just leave it stale.
- Don't create an Ideas or Bugs note for something that's really an implementation detail of an existing feature — that belongs in the feature note's Capabilities or Implementation section instead.
