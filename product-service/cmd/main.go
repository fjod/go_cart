package main

import (
	"log"
	"net"

	repository "github.com/fjod/go_cart/product-service/internal/db"
	grpcHandler "github.com/fjod/go_cart/product-service/internal/grpc"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("Product-service started")
	repo, err := repository.NewRepository("./internal/db/products.db")
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	// Run migrations
	if err := repo.RunMigrations("./internal/db/migrations"); err != nil {
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
	listener, err := net.Listen("tcp", ":8084")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Println("Product service listening on :8084")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
