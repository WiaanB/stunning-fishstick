# Overview

South African minibus taxi platform, built as a portfolio/learning project.

## Surfaces
- **Passenger app** — pre-booking and pre-payment before taxi arrival
- **Driver app** — trip acceptance and boarding verification via unique trip codes
- **Fleet owner web dashboard** — vehicle tracking, income, per-vehicle statistics

## Structure
Monorepo with four apps (passenger, driver, fleet dashboard, backend API) + shared packages for types and utilities.

## Tools & Stack
- **Backend**: Go, PostgreSQL (pgx/sqlc), Redis, outbox pattern for event dispatch
- **Mobile**: Kotlin Multiplatform (KMP), SwiftUI (iOS), Jetpack Compose (Android)
- **Web dashboard**: React/TypeScript
- **Contracts**: Protobuf
- **Payments**: PayShap
- **Infra pattern**: Monorepo with shared packages

See [[02 Architecture Principles]], [[Backend Scaffolding]], [[Roadmap]].
