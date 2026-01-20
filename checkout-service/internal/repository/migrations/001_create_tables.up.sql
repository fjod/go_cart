CREATE TABLE checkout_sessions (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE checkout_sessions IS 'Stores checkout session state during the purchase flow';
COMMENT ON COLUMN checkout_sessions.id IS 'Unique checkout session identifier';
COMMENT ON COLUMN checkout_sessions.user_id IS 'User who initiated checkout';
COMMENT ON COLUMN checkout_sessions.status IS 'Session state: pending | completed | failed | cancelled';
COMMENT ON COLUMN checkout_sessions.total_amount IS 'Final checkout amount in base currency';
COMMENT ON COLUMN checkout_sessions.created_at IS 'Session creation timestamp';
COMMENT ON COLUMN checkout_sessions.updated_at IS 'Session status update timestamp';

CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP
);

COMMENT ON TABLE outbox_events IS 'Transactional Outbox Pattern: events written atomically with business data, then published to Kafka';
COMMENT ON COLUMN outbox_events.id IS 'Auto-incrementing event ID for ordering';
COMMENT ON COLUMN outbox_events.aggregate_id IS 'References checkout_sessions.id - the entity this event belongs to';
COMMENT ON COLUMN outbox_events.event_type IS 'Event name: checkout.created | checkout.completed | checkout.failed | checkout.cancelled';
COMMENT ON COLUMN outbox_events.payload IS 'Event data as JSON: {checkout_id, user_id, items[], total_amount, timestamp}, basically a snapshot of cart state';
COMMENT ON COLUMN outbox_events.created_at IS 'When event was written to outbox';
COMMENT ON COLUMN outbox_events.processed_at IS 'NULL while pending; set to timestamp when published to Kafka';

CREATE INDEX idx_outbox_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;
COMMENT ON INDEX idx_outbox_unprocessed IS 'Partial index for efficient polling of unprocessed events';

-- Foreign key linking the event to its associated checkout session
ALTER TABLE outbox_events ADD CONSTRAINT outbox_FK FOREIGN KEY (aggregate_id) REFERENCES checkout_sessions(id);
