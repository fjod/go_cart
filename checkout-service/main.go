package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	checkoutgrpc "github.com/fjod/go_cart/checkout-service/internal/grpc"
	"github.com/fjod/go_cart/checkout-service/internal/repository"
	"github.com/fjod/go_cart/checkout-service/internal/service"
	pb "github.com/fjod/go_cart/checkout-service/pkg/proto"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	inventorypb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	paymentpb "github.com/fjod/go_cart/payment-service/pkg/proto"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("checkout-service starting...")

	// Configuration
	grpcPort := getEnv("GRPC_PORT", "50056")
	cartServiceAddr := getEnv("CART_SERVICE_ADDR", "localhost:50052")
	productServiceAddr := getEnv("PRODUCT_SERVICE_ADDR", "localhost:50051")
	inventoryServiceAddr := getEnv("INVENTORY_SERVICE_ADDR", "localhost:50053")
	paymentServiceAddr := getEnv("PAYMENT_SERVICE_ADDR", "localhost:50054")
	requestTimeout := 5 * time.Second

	// Database setup
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "ecommerce")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./internal/repository/migrations")

	port, err := strconv.Atoi(dbPort)
	if err != nil {
		log.Fatalf("Invalid DB_PORT: %v", err)
	}

	creds := &repository.Credentials{
		Host:              dbHost,
		Port:              port,
		User:              dbUser,
		Password:          dbPass,
		DBName:            dbName,
		MigrationsDirPath: migrationsPath,
	}

	repo, err := repository.NewRepository(creds)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	if err := repo.RunMigrations(creds); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	// Create gRPC client connections
	cartConn, err := grpc.NewClient(cartServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to cart service: %v", err)
	}
	defer cartConn.Close()
	cartClient := cartpb.NewCartServiceClient(cartConn)
	log.Printf("Connected to cart service at %s", cartServiceAddr)

	productConn, err := grpc.NewClient(productServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to product service: %v", err)
	}
	defer productConn.Close()
	productClient := productpb.NewProductServiceClient(productConn)
	log.Printf("Connected to product service at %s", productServiceAddr)

	inventoryConn, err := grpc.NewClient(inventoryServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to inventory service: %v", err)
	}
	defer inventoryConn.Close()
	inventoryClient := inventorypb.NewInventoryServiceClient(inventoryConn)
	log.Printf("Connected to inventory service at %s", inventoryServiceAddr)

	paymentConn, err := grpc.NewClient(paymentServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to payment service: %v", err)
	}
	defer paymentConn.Close()
	paymentClient := paymentpb.NewPaymentServiceClient(paymentConn)
	log.Printf("Connected to payment service at %s", paymentServiceAddr)

	// Create handler wrappers
	cartHandler := service.NewCartHandler(cartClient, requestTimeout)
	productHandler := service.NewProductHandler(productClient, requestTimeout)
	inventoryHandler := service.NewInventoryHandler(inventoryClient, requestTimeout)
	paymentHandler := service.NewPaymentHandler(paymentClient, requestTimeout)

	// Instantiate checkout service
	checkoutService := service.NewCheckoutService(
		repo,
		cartHandler,
		productHandler,
		inventoryHandler,
		paymentHandler,
	)

	// Create gRPC handler
	checkoutServer := checkoutgrpc.NewCheckoutServiceServer(checkoutService)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterCheckoutServiceServer(grpcServer, checkoutServer)
	reflection.Register(grpcServer)

	go func() {
		log.Printf("Checkout service listening on :%s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down checkout service...")
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx // Used for cleanup

	log.Println("Checkout service stopped")
}
