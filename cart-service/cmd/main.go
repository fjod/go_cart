package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	c "github.com/fjod/go_cart/cart-service/internal/cache"
	cartgrpc "github.com/fjod/go_cart/cart-service/internal/grpc"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	s "github.com/fjod/go_cart/cart-service/internal/service"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/redis/go-redis/v9"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Configuration
	cartServicePort := getEnv("CART_SERVICE_PORT", "50052")
	productServiceAddr := getEnv("PRODUCT_SERVICE_ADDR", "localhost:50051")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGO_DB_NAME", "cartdb")

	// Set up MongoDB connection
	ctx := context.Background()
	mongoDB, err := repository.ConnectMongoDB(ctx, mongoURI, mongoDBName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Create repository
	repo := repository.NewMongoRepository(mongoDB)
	log.Printf("Connected to MongoDB at %s", mongoURI)

	// Set up gRPC connection to Product Service
	productConn, err := grpc.NewClient(
		productServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to product service: %v", err)
	}
	defer productConn.Close()

	productClient := productpb.NewProductServiceClient(productConn)
	log.Printf("Connected to product service at %s", productServiceAddr)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}
	log.Printf("Redis ping succeeded")

	cache := c.NewRedisCache(redisClient)
	service := s.NewCartService(repo, cache)
	cartServer := cartgrpc.NewCartServiceServer(service, productClient)

	// Set up gRPC server for cart service
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cartServicePort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterCartServiceServer(grpcServer, cartServer)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	// Graceful shutdown
	go func() {
		log.Printf("Cart service listening on port %s", cartServicePort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down cart service...")
	grpcServer.GracefulStop()
	mongoDB.Client().Disconnect(ctx)
	log.Println("Cart service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
