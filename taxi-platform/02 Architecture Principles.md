# Architecture Principles

Core values: **Domain-Driven Design (DDD)**, **event-driven architecture**, strong **type safety** throughout.

## Patterns
- Command → domain method → event → handler, throughout the backend
- DDD with domain packages owning their own state machines and events
- Event bus designed as swappable infrastructure (in-process now, NATS/Kafka later)
- Vertical slice approach for early integration: one full end-to-end slice before building breadth
- Mock clients (`cmd/mockclient/`) enable load testing without requiring real mobile apps

## Related
- [[03 Domain - Trip State Machine]]
- [[04 Backend Scaffolding]]
- [[08 Learnings and Principles]]
