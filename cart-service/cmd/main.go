package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	c "github.com/fjod/go_cart/cart-service/internal/cache"
	cartgrpc "github.com/fjod/go_cart/cart-service/internal/grpc"
	poller2 "github.com/fjod/go_cart/cart-service/internal/poller"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	s "github.com/fjod/go_cart/cart-service/internal/service"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	log := logger.New("cart-service", "info")

	// Configuration
	cartServicePort := getEnv("CART_SERVICE_PORT", "50052")
	productServiceAddr := getEnv("PRODUCT_SERVICE_ADDR", "localhost:50051")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGO_DB_NAME", "cartdb")

	// Set up MongoDB connection
	ctx := context.Background()
	mongoDB, err := repository.ConnectMongoDB(ctx, mongoURI, mongoDBName)
	if err != nil {
		log.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}

	// Create repository
	repo := repository.NewMongoRepository(mongoDB)
	log.Info("connected to MongoDB", "uri", mongoURI)

	// Set up gRPC connection to Product Service
	productConn, err := grpc.NewClient(
		productServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to product service", "addr", productServiceAddr, "error", err)
		os.Exit(1)
	}
	defer productConn.Close()

	productClient := productpb.NewProductServiceClient(productConn)
	log.Info("connected to product service", "addr", productServiceAddr)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	log.Info("redis ping succeeded")

	cache := c.NewRedisCache(redisClient)
	service := s.NewCartService(repo, cache, log)
	cartServer := cartgrpc.NewCartServiceServer(service, productClient, log)

	// Set up gRPC server for cart service
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cartServicePort))
	if err != nil {
		log.Error("failed to listen", "port", cartServicePort, "error", err)
		os.Exit(1)
	}

	shutdown, err := tracing.InitTracer("cart-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterCartServiceServer(grpcServer, cartServer)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	kafkaPort := getEnv("KAFKA_ADDR", "localhost:9092")
	poller := poller2.NewPoller(repo, cache, log, kafkaPort)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	pollerCtx, pollerCancel := context.WithCancel(ctx)
	go func() {
		poller.Run(pollerCtx)
		wg.Done()
	}()

	chWait := make(chan struct{})
	go func() {
		wg.Wait()
		close(chWait)
	}()

	// Graceful shutdown
	go func() {
		log.Info("cart service listening", "port", cartServicePort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down cart service")
	grpcServer.GracefulStop()
	pollerCancel()

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mongoDB.Client().Disconnect(timeoutCtx)

	poller.Close()
	select {
	case <-chWait:
		log.Info("poller stopped")
	case <-timeoutCtx.Done():
		log.Warn("poller did not stop within timeout")
	}

	log.Info("cart service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
