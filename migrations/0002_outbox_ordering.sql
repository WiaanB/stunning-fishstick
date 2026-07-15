-- Outbox ordering and cross-instance dispatch fixes: created_at alone has
-- no deterministic tiebreak for same-timestamp rows, and a claim lease lets
-- dispatcher instances see a row is already being handled elsewhere without
-- holding its FOR UPDATE lock for the duration of publish (which can now
-- block on a handler). See
-- taxi-platform/Planning/Bugs/event-system-reliability-fixes.md.

ALTER TABLE outbox_events
    ADD COLUMN seq bigserial,
    ADD COLUMN claimed_at timestamptz;

DROP INDEX idx_outbox_events_undispatched;

CREATE INDEX idx_outbox_events_undispatched
    ON outbox_events (created_at, seq)
    WHERE dispatched_at IS NULL;

-- Supports the "is this aggregate already claimed elsewhere" correlated
-- subquery in Dispatcher's claim query.
CREATE INDEX idx_outbox_events_aggregate_claimed
    ON outbox_events (aggregate_id)
    WHERE dispatched_at IS NULL AND claimed_at IS NOT NULL;
