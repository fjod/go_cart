CREATE TABLE checkout_sessions (
                                   id UUID PRIMARY KEY,
                                   user_id VARCHAR(255) NOT NULL,
                                   cart_snapshot JSONB NOT NULL,
                                   status VARCHAR(50) NOT NULL,
                                   idempotency_key VARCHAR(255) UNIQUE NOT NULL,

    -- Saga state tracking
                                   inventory_reservation_id VARCHAR(255),
                                   payment_id VARCHAR(255),
    -- NO order_id - Orders Service owns that relationship

    -- Metadata
                                   total_amount DECIMAL(10, 2) NOT NULL,
                                   currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Timestamps
                                   created_at TIMESTAMP NOT NULL DEFAULT NOW(),
                                   updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checkout_idempotency ON checkout_sessions(idempotency_key);
CREATE INDEX idx_checkout_user ON checkout_sessions(user_id);
CREATE INDEX idx_checkout_status ON checkout_sessions(status);

COMMENT ON TABLE checkout_sessions IS 'Stores checkout session state during the purchase flow';
COMMENT ON COLUMN checkout_sessions.cart_snapshot IS 'Snapshot of cart items at checkout time for audit and compensation';
COMMENT ON COLUMN checkout_sessions.idempotency_key IS 'Client-provided unique key to prevent duplicate checkouts';
COMMENT ON COLUMN checkout_sessions.inventory_reservation_id IS 'Reservation ID from Inventory Service';
COMMENT ON COLUMN checkout_sessions.payment_id IS 'Payment transaction ID from Payment Service';

CREATE TABLE outbox_events (
                               id BIGSERIAL PRIMARY KEY,
                               aggregate_id UUID NOT NULL,
                               event_type VARCHAR(100) NOT NULL,
                               payload JSONB NOT NULL,
                               created_at TIMESTAMP NOT NULL DEFAULT NOW(),
                               processed_at TIMESTAMP
);

CREATE INDEX idx_outbox_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;

ALTER TABLE outbox_events
    ADD CONSTRAINT fk_outbox_checkout
        FOREIGN KEY (aggregate_id)
            REFERENCES checkout_sessions(id);

COMMENT ON TABLE outbox_events IS 'Transactional Outbox Pattern: events written atomically with business data';
COMMENT ON COLUMN outbox_events.payload IS 'Event data published to Kafka: includes checkout_id, user_id, items, total_amount, timestamp';