package main

import (
	"log"
	"net"
	"os"

	grpcHandler "github.com/fjod/go_cart/product-service/internal/grpc"
	repository "github.com/fjod/go_cart/product-service/internal/repository"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("Product-service started")

	// Use environment variables with sensible defaults
	dbPath := getEnv("DB_PATH", "./internal/repository/products.db")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./internal/repository/migrations")

	repo, err := repository.NewRepository(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	// Run migrations
	if err := repo.RunMigrations(migrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Migrations completed successfully")

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register product service
	productService := grpcHandler.NewProductServiceServer(repo)
	pb.RegisterProductServiceServer(grpcServer, productService)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	// Start listening
	port := getEnv("GRPC_PORT", ":50051")
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Product service listening on %s", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
