# Key Learnings & Principles

- **Outbox pattern from day one** — atomicity between state change and event emission is foundational: trip row update + outbox event row commit in a single transaction, with a separate polling dispatcher. Adding this later would be costly.
- **Async over sync dispatch** — event bus uses async worker-pool pattern rather than synchronous dispatch, consistent with event-driven goals.
- **Separation of write model and live state** — Postgres (via sqlc) for durable write model; Redis for live state (GPS, occupancy).
- **Protobuf as contract boundary** — single source of truth across the stack prevents type drift between Go, Kotlin/TypeScript clients.
- **Shared logic, native UI** — KMP for mobile with fully native UI (SwiftUI/iOS, Jetpack Compose/Android), shared ViewModel/StateFlow pattern — not shared UI code.
- **Translation source of truth** — see [[06 Translations]].

## Related
- [[02 Architecture Principles]]
- [[04 Backend Scaffolding]]
