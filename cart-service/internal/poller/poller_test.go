package poller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	c "github.com/fjod/go_cart/cart-service/internal/cache"
	"github.com/fjod/go_cart/cart-service/internal/domain"
	r "github.com/fjod/go_cart/cart-service/internal/repository"
	"github.com/redis/go-redis/v9"
	kafkaGo "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"gotest.tools/v3/assert"
)

func setupTestRedis(t *testing.T) (*c.RedisCache, *miniredis.Miniredis, func()) {
	// Create an in-memory Redis server
	mr := miniredis.RunT(t)

	// Create Redis client pointing to miniredis
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create cache instance
	cache := c.NewRedisCache(client)

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return cache, mr, cleanup
}

func setupTestDB(t *testing.T) (r.CartRepository, func()) {
	ctx := context.Background()

	// Start MongoDB container
	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)

	// Get connection string
	uri, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Connect to MongoDB
	db, err := r.ConnectMongoDB(ctx, uri, "testdb")
	require.NoError(t, err)

	// Create repository
	repo := r.NewMongoRepository(db)

	cleanup := func() {
		if err := mongoContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}

	return repo, cleanup
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

func TestPoller_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache, _, cleanupRedis := setupTestRedis(t)
	defer cleanupRedis()
	dbRepo, cleanupDb := setupTestDB(t)
	defer cleanupDb()
	brokers, cleanupKafka := setupKafka(t)
	defer cleanupKafka()
	topic := "checkout-outbox"
	createTopic(t, brokers, topic)

	poller := NewPoller(dbRepo, cache, brokers)

	// create cart and cache it
	dbRepo.AddItem(ctx, "123", domain.CartItem{
		ProductID: 1,
		Quantity:  1,
		AddedAt:   time.Time{},
	})
	cart, errGetCart := dbRepo.GetCart(ctx, "123")
	require.NoError(t, errGetCart)
	require.NotNil(t, cart)
	assert.Equal(t, 1, len(cart.Items))
	err := cache.Set(ctx, "123", cart)
	require.NoError(t, err)

	w := &kafkaGo.Writer{
		Addr:                   kafkaGo.TCP(brokers),
		Topic:                  "checkout-outbox",
		Balancer:               &kafkaGo.LeastBytes{},
		AllowAutoTopicCreation: true,
	}

	payload := map[string]interface{}{
		"checkout_id":  "chId",
		"user_id":      "123",
		"items":        "{}",
		"total_amount": "1",
		"currency":     "rur",
		"completed_at": time.Time{},
	}

	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	msg := kafkaGo.Message{
		Key:   []byte("chId"), // checkout_id for ordering
		Value: payloadJSON,    // Already JSON from database
		Headers: []kafkaGo.Header{
			{Key: "event_type", Value: []byte("checkout")},
		},
	}

	err = w.WriteMessages(ctx, msg)
	require.NoError(t, err)
	w.Close()

	go poller.Run(ctx) // start poller
	require.Eventually(t, func() bool {
		_, eClearCart := dbRepo.GetCart(ctx, "123")
		return errors.Is(eClearCart, r.ErrCartNotFound) // cart is cleared
	}, 15*time.Second, 500*time.Millisecond)

	require.Eventually(t, func() bool {
		_, eGetCache := cache.Get(ctx, "123")
		return errors.Is(eGetCache, c.ErrCacheMiss) // cache is cleared
	}, 15*time.Second, 500*time.Millisecond)

	fmt.Println("Poller run finished")
}
