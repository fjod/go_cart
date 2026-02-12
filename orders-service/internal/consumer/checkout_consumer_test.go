package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fjod/go_cart/orders-service/internal/repository"
	"github.com/google/uuid"
	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupKafka(t *testing.T) (string, func()) {
	ctx := context.Background()

	kafkaContainer, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err)

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)

	cleanup := func() {
		if err := kafkaContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate kafka container: %v", err)
		}
	}

	return brokers[0], cleanup
}

func setupPostgres(t *testing.T) (repository.OrderRepository, func()) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	creds := &repository.Credentials{
		Host:              host,
		Port:              port.Int(),
		User:              "testuser",
		Password:          "testpass",
		DBName:            "testdb",
		MigrationsDirPath: "../repository/migrations",
	}

	repo, err := repository.NewRepository(creds)
	require.NoError(t, err)

	err = repo.RunMigrations(creds)
	require.NoError(t, err)

	cleanup := func() {
		repo.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %s", err)
		}
	}

	return repo, cleanup
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

func writeEvent(t *testing.T, brokerAddr string, event CheckoutCompletedEvent) {
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	w := &kafkaGo.Writer{
		Addr:                   kafkaGo.TCP(brokerAddr),
		Topic:                  "checkout-outbox",
		Balancer:               &kafkaGo.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer w.Close()

	msg := kafkaGo.Message{
		Key:   []byte(event.CheckoutID),
		Value: payload,
		Headers: []kafkaGo.Header{
			{Key: "event_type", Value: []byte("CheckoutCompleted")},
		},
	}

	err = w.WriteMessages(context.Background(), msg)
	require.NoError(t, err)
}

func TestProcessMessage_CreatesOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	brokerAddr, cleanupKafka := setupKafka(t)
	defer cleanupKafka()

	repo, cleanupPostgres := setupPostgres(t)
	defer cleanupPostgres()

	topic := "checkout-outbox"
	createTopic(t, brokerAddr, topic)

	checkoutID := uuid.New()
	event := CheckoutCompletedEvent{
		CheckoutID:  checkoutID.String(),
		UserID:      "user-test-1",
		TotalAmount: 129.99,
		Currency:    "USD",
		Items: []eventItem{
			{ProductID: 1, ProductName: "Laptop", Quantity: 1, Price: 129.99},
		},
	}

	writeEvent(t, brokerAddr, event)

	c := NewConsumer(repo, brokerAddr)
	go c.Run(ctx)

	require.Eventually(t, func() bool {
		orders, err := repo.ListOrdersByUserID(ctx, "user-test-1")
		if err != nil || len(orders) == 0 {
			return false
		}
		return orders[0].CheckoutID == checkoutID
	}, 15*time.Second, 500*time.Millisecond)
}

func TestProcessMessage_Idempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	brokerAddr, cleanupKafka := setupKafka(t)
	defer cleanupKafka()

	repo, cleanupPostgres := setupPostgres(t)
	defer cleanupPostgres()

	topic := "checkout-outbox"
	createTopic(t, brokerAddr, topic)

	checkoutID := uuid.New()
	event := CheckoutCompletedEvent{
		CheckoutID:  checkoutID.String(),
		UserID:      "user-idem-test",
		TotalAmount: 29.99,
		Currency:    "USD",
		Items: []eventItem{
			{ProductID: 2, ProductName: "Mouse", Quantity: 1, Price: 29.99},
		},
	}

	// Write same event twice
	writeEvent(t, brokerAddr, event)
	writeEvent(t, brokerAddr, event)

	c := NewConsumer(repo, brokerAddr)
	go c.Run(ctx)

	// Wait for at least one order to be created
	require.Eventually(t, func() bool {
		orders, err := repo.ListOrdersByUserID(ctx, "user-idem-test")
		return err == nil && len(orders) > 0
	}, 15*time.Second, 500*time.Millisecond)

	// Give consumer time to process duplicate
	time.Sleep(2 * time.Second)

	orders, err := repo.ListOrdersByUserID(ctx, "user-idem-test")
	require.NoError(t, err)
	require.Len(t, orders, 1, "should only have one order despite duplicate messages")
}
