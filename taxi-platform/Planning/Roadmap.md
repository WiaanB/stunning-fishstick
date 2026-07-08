# Roadmap

Agreed build sequence:

1. HTTP handlers in `internal/api`
2. sqlc-generated queries
3. Redis occupancy state (live GPS location and occupancy)
4. Protobuf contracts as single source of truth (generating types for Go, Kotlin, TypeScript)
5. One vertical slice end-to-end with mocked maps and payment providers
6. KMP shared module and ViewModels
7. Layering in fleet dashboard, multi-seat, and translations

## Open items
- Native speaker review needed for several SA language translations — see [[Translations]]
- [[Payments]] — PayShap selected as primary rail, integration not yet started

## Related
- [[Backend Scaffolding]]
