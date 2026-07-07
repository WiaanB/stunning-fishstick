-- Trips and the transactional outbox. See taxi-platform/04 Backend
-- Scaffolding.md and taxi-platform/08 Learnings and Principles.md — the
-- outbox row is written in the same transaction as the trip row so state
-- changes and event emission can never drift apart.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE trips (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state       text NOT NULL,
    seat_count  integer NOT NULL CHECK (seat_count >= 1),
    vehicle_id  uuid,
    code        text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Supports the occupancy floor invariant: sum(seat_count) for all
-- in_progress trips per vehicle.
CREATE INDEX idx_trips_vehicle_in_progress
    ON trips (vehicle_id)
    WHERE state = 'in_progress';

CREATE TABLE outbox_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id   uuid NOT NULL,
    event_type     text NOT NULL,
    payload        jsonb NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    dispatched_at  timestamptz
);

-- Polling dispatcher scans undispatched rows in creation order; partial
-- index keeps that scan cheap as dispatched rows accumulate.
CREATE INDEX idx_outbox_events_undispatched
    ON outbox_events (created_at)
    WHERE dispatched_at IS NULL;
