package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	d "github.com/fjod/go_cart/checkout-service/domain"
	r "github.com/fjod/go_cart/checkout-service/internal/repository"
	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

type MockRepository struct {
	GetKey                    *string
	GetStatus                 *d.CheckoutStatus
	GetErr                    error
	CreateErr                 error
	CreatedSession            *r.CheckoutSession // Captures the session passed to CreateCheckoutSession
	ReservationId             *string
	PaymentId                 *string
	OutboxId                  *string
	StuckSessions             []*r.CheckoutSession
	GetStuckSessionsErr       error
	CompleteCheckoutErr       error
	CompletedCheckoutIDs      []string // Track all completed sessions
	CompleteCheckoutCallCount int      // Track how many times CompleteCheckoutSession was called
	OutboxEvents              []*r.OutboxEvent
	ProcessedId               int
}

func (m *MockRepository) Close() error {
	return nil
}

func (m *MockRepository) RunMigrations(*r.Credentials) error {
	return nil
}

func (m *MockRepository) GetCheckoutSessionByIdempotencyKey(_ context.Context, _ string) (*string, *d.CheckoutStatus, error) {
	return m.GetKey, m.GetStatus, m.GetErr
}

func (m *MockRepository) CreateCheckoutSession(_ context.Context, session *r.CheckoutSession) error {
	m.CreatedSession = session
	return m.CreateErr
}

func (m *MockRepository) UpdateCheckoutSessionStatus(_ context.Context, _ *string, _ *d.CheckoutStatus) error {
	return nil
}

func (m *MockRepository) SetReservation(_ context.Context, _ *string, _ *d.CheckoutStatus, reserveId *string) error {
	m.ReservationId = reserveId
	return nil
}

func (m *MockRepository) SetPayment(_ context.Context, _ *string, _ *d.CheckoutStatus, payId *string) error {
	m.PaymentId = payId
	return nil
}

func (m *MockRepository) CompleteCheckoutSession(_ context.Context, id *string, _ []byte, _ *d.CheckoutStatus) error {
	m.CompleteCheckoutCallCount++
	if m.CompleteCheckoutErr != nil {
		return m.CompleteCheckoutErr
	}
	m.OutboxId = id
	m.CompletedCheckoutIDs = append(m.CompletedCheckoutIDs, *id)
	return nil
}

func (m *MockRepository) GetUnprocessedEvents(context.Context, int) ([]*r.OutboxEvent, error) {
	if len(m.OutboxEvents) > 0 {
		ev := []*r.OutboxEvent{m.OutboxEvents[0]} // Return first event once
		m.OutboxEvents = []*r.OutboxEvent{}
		return ev, nil
	}
	return m.OutboxEvents, nil
}

func (m *MockRepository) MarkEventAsProcessed(_ context.Context, id int) error {
	m.ProcessedId = id
	return nil
}

func (m *MockRepository) GetStuckSessions(context.Context) ([]*r.CheckoutSession, error) {
	if m.GetStuckSessionsErr != nil {
		return nil, m.GetStuckSessionsErr
	}
	return m.StuckSessions, nil
}

func setupKafka(t *testing.T) (string, func()) {
	ctx := context.Background()

	// Start Kafka container using testcontainers Kafka module
	kafkaContainer, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err)

	// Get broker address
	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers, "broker address should not be empty")

	cleanup := func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate kafka container: %v", err)
		}
	}

	return brokers[0], cleanup
}

func createTopic(t *testing.T, brokerAddr, topic string) {
	conn, err := kafkaGo.Dial("tcp", brokerAddr)
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafkaGo.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	require.NoError(t, err)
	defer controllerConn.Close()

	topicConfigs := []kafkaGo.TopicConfig{{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		t.Logf("topic creation error (may already exist): %v", err)
	}
}

func TestOutboxPoller_PublishesEventsToKafka(t *testing.T) {
	brokerAddr, cleanup := setupKafka(t)
	defer cleanup()

	// Create test topic
	createTopic(t, brokerAddr, "checkout-outbox")

	// Give Kafka time to fully initialize the topic
	time.Sleep(5 * time.Second)

	// Setup mock repository with unprocessed events
	mockRepo := &MockRepository{
		OutboxEvents: []*r.OutboxEvent{
			{
				ID:          1,
				AggregateId: "checkout-123",
				EventType:   "CheckoutCompleted",
				Payload:     json.RawMessage(`{"checkout_id":"checkout-123","user_id":"user-456"}`),
				CreatedAt:   time.Now(),
			},
		},
		StuckSessions: []*r.CheckoutSession{},
	}

	// Create poller with real Kafka writer
	writer := &kafkaGo.Writer{
		Addr:         kafkaGo.TCP(brokerAddr),
		Topic:        "checkout-outbox",
		Balancer:     &kafkaGo.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	defer writer.Close()

	poller := &OutboxPoller{
		timeout:      5 * time.Second,
		eventTick:    1 * time.Second,
		recoveryTick: 5 * time.Second,
		repo:         mockRepo,
		writer:       writer,
	}

	// Process events with longer timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go poller.Run(ctx)

	// Verify message was written to Kafka
	reader := kafkaGo.NewReader(kafkaGo.ReaderConfig{
		Brokers:  []string{brokerAddr},
		Topic:    "checkout-outbox",
		GroupID:  "test-consumer",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	msg, err := reader.ReadMessage(ctx)
	require.NoError(t, err)

	assert.Equal(t, "checkout-123", string(msg.Key))

	var payload map[string]interface{}
	err = json.Unmarshal(msg.Value, &payload)
	require.NoError(t, err)

	assert.Equal(t, "checkout-123", payload["checkout_id"])
	assert.Equal(t, "user-456", payload["user_id"])
	// Verify event was marked as processed
	assert.Equal(t, mockRepo.ProcessedId, 1)
}

func TestRecoveringStuckSession(t *testing.T) {
	items := make([]d.CartSnapshotItem, 2)
	items[0] = d.CartSnapshotItem{
		ProductID: 1,
		Quantity:  1,
	}
	items[1] = d.CartSnapshotItem{
		ProductID: 2,
		Quantity:  2,
	}

	snapshot := &d.CartSnapshot{
		Items:      items,
		Currency:   "USD",
		CapturedAt: time.Now(),
	}
	snapshotJSON, _ := json.Marshal(snapshot)
	s := &r.CheckoutSession{
		ID:                     "checkout-id-1",
		UserID:                 "userId",
		CartSnapshot:           snapshotJSON,
		Status:                 "status",
		IdempotencyKey:         "key",
		InventoryReservationID: nil,
		PaymentID:              nil,
		TotalAmount:            "123",
		Currency:               "USD",
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	sessions := []*r.CheckoutSession{s}
	mockRepo := &MockRepository{
		StuckSessions: sessions,
	}

	poller := NewOutboxPoller(mockRepo)
	poller.recoverStuckSessions(context.Background())
	require.Equal(t, "checkout-id-1", *mockRepo.OutboxId)
}

func TestRecoveringStuckSession_GetStuckSessionsError(t *testing.T) {
	// Test that repository errors are handled gracefully
	mockRepo := &MockRepository{
		GetStuckSessionsErr: errors.New("database connection error"),
	}

	poller := NewOutboxPoller(mockRepo)

	// Should not panic, just log error and return
	poller.recoverStuckSessions(context.Background())

	// Verify no sessions were attempted to be completed
	assert.Equal(t, 0, mockRepo.CompleteCheckoutCallCount)
}

func TestRecoveringStuckSession_EmptySessionsList(t *testing.T) {
	// Test that empty session list is handled gracefully
	mockRepo := &MockRepository{
		StuckSessions: []*r.CheckoutSession{}, // Empty list
	}

	poller := NewOutboxPoller(mockRepo)

	// Should not panic, just return without doing anything
	poller.recoverStuckSessions(context.Background())

	// Verify no sessions were attempted to be completed
	assert.Equal(t, 0, mockRepo.CompleteCheckoutCallCount)
}

func TestRecoveringStuckSession_InvalidCartSnapshot(t *testing.T) {
	// Test that corrupted cart snapshot JSON is handled gracefully
	session := &r.CheckoutSession{
		ID:           "checkout-bad-json",
		UserID:       "user123",
		CartSnapshot: []byte(`{invalid json here!`), // Malformed JSON
		Status:       "PAYMENT_COMPLETED",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mockRepo := &MockRepository{
		StuckSessions: []*r.CheckoutSession{session},
	}

	poller := NewOutboxPoller(mockRepo)

	// Should not panic - should log error and skip this session
	poller.recoverStuckSessions(context.Background())

	// ✅ FIXED: Code now properly skips session on unmarshal error
	assert.Equal(t, 0, mockRepo.CompleteCheckoutCallCount,
		"Session with invalid JSON should be skipped, not processed")
}

func TestRecoveringStuckSession_CompleteCheckoutError(t *testing.T) {
	// This is what the test SHOULD do once the bug is fixed:
	items := []d.CartSnapshotItem{
		{ProductID: 1, Quantity: 1},
	}
	snapshot := &d.CartSnapshot{
		Items:      items,
		Currency:   "USD",
		CapturedAt: time.Now(),
	}
	snapshotJSON, _ := json.Marshal(snapshot)

	session := &r.CheckoutSession{
		ID:           "checkout-id-fail",
		UserID:       "user123",
		CartSnapshot: snapshotJSON,
		Status:       "PAYMENT_COMPLETED",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mockRepo := &MockRepository{
		StuckSessions:       []*r.CheckoutSession{session},
		CompleteCheckoutErr: errors.New("database deadlock"),
	}

	poller := NewOutboxPoller(mockRepo)

	// Should NOT exit the process - should log error and continue
	poller.recoverStuckSessions(context.Background())

	// Verify the call was attempted but handled gracefully
	assert.Equal(t, 1, mockRepo.CompleteCheckoutCallCount)
}

func TestRecoveringStuckSession_MultipleSessionsWithPartialFailures(t *testing.T) {
	// Test that one failing session doesn't prevent others from being processed

	// Session 1: Valid session (should succeed)
	snapshot1 := &d.CartSnapshot{
		Items:      []d.CartSnapshotItem{{ProductID: 1, Quantity: 1}},
		Currency:   "USD",
		CapturedAt: time.Now(),
	}
	json1, _ := json.Marshal(snapshot1)
	session1 := &r.CheckoutSession{
		ID:           "checkout-success-1",
		UserID:       "user1",
		CartSnapshot: json1,
		Status:       "PAYMENT_COMPLETED",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Session 2: Invalid JSON (should be skipped)
	session2 := &r.CheckoutSession{
		ID:           "checkout-bad-json",
		UserID:       "user2",
		CartSnapshot: []byte(`{corrupted`),
		Status:       "PAYMENT_COMPLETED",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Session 3: Valid session (should succeed)
	snapshot3 := &d.CartSnapshot{
		Items:      []d.CartSnapshotItem{{ProductID: 3, Quantity: 2}},
		Currency:   "USD",
		CapturedAt: time.Now(),
	}
	json3, _ := json.Marshal(snapshot3)
	session3 := &r.CheckoutSession{
		ID:           "checkout-success-2",
		UserID:       "user3",
		CartSnapshot: json3,
		Status:       "PAYMENT_COMPLETED",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mockRepo := &MockRepository{
		StuckSessions:        []*r.CheckoutSession{session1, session2, session3},
		CompletedCheckoutIDs: []string{},
	}

	poller := NewOutboxPoller(mockRepo)
	poller.recoverStuckSessions(context.Background())

	// ✅ FIXED: Error handling now works correctly
	// - Session 1 completes successfully
	// - Session 2 is skipped (bad JSON)
	// - Session 3 completes successfully
	// Total: 2 successful completions

	require.Len(t, mockRepo.CompletedCheckoutIDs, 2, "Should complete 2 valid sessions")
	assert.Contains(t, mockRepo.CompletedCheckoutIDs, "checkout-success-1")
	assert.Contains(t, mockRepo.CompletedCheckoutIDs, "checkout-success-2")
	assert.NotContains(t, mockRepo.CompletedCheckoutIDs, "checkout-bad-json",
		"Session with corrupted JSON should be skipped")
}

func TestRecoveringStuckSession_NilSessionsList(t *testing.T) {
	// Test that nil sessions list doesn't cause panic
	mockRepo := &MockRepository{
		StuckSessions: nil, // Nil instead of empty slice
	}

	poller := NewOutboxPoller(mockRepo)

	// Should not panic
	poller.recoverStuckSessions(context.Background())

	assert.Equal(t, 0, mockRepo.CompleteCheckoutCallCount)
}
