package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	cart_grpc "github.com/fjod/go_cart/cart-service/internal/grpc"
	"github.com/fjod/go_cart/cart-service/internal/repository"
	pb "github.com/fjod/go_cart/cart-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
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

	// Create cart service server with both repository and product client
	cartServer := cart_grpc.NewCartServiceServer(repo, productClient)

	// Set up gRPC server for cart service
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cartServicePort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAddCartItemServiceServer(grpcServer, cartServer)

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
