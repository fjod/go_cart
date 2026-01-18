package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	inventorygrpc "github.com/fjod/go_cart/inventory-service/internal/grpc"
	"github.com/fjod/go_cart/inventory-service/internal/store"
	pb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Initial stock levels matching product-service seeds
var initialStock = map[int64]int32{
	1: 100, // Laptop
	2: 500, // Mouse
	3: 300, // Keyboard
	4: 150, // Monitor
	5: 200, // Headphones
}

func main() {
	port := getEnv("INVENTORY_SERVICE_PORT", "50053")

	// Create in-memory store
	memStore := store.NewMemoryStore()

	// Initialize stock levels
	for productID, quantity := range initialStock {
		if err := memStore.SetStock(productID, quantity); err != nil {
			log.Fatalf("Failed to set initial stock for product %d: %v", productID, err)
		}
	}
	log.Printf("Initialized stock for %d products", len(initialStock))

	// Create gRPC server
	server := inventorygrpc.NewInventoryServiceServer(memStore)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, server)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	// Start server in goroutine
	go func() {
		log.Printf("Inventory service listening on port %s", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down inventory service...")
	grpcServer.GracefulStop()
	err = memStore.Close()
	if err != nil {
		log.Fatalf("Failed to stop memstore: %v", err)
	}
	log.Println("Inventory service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
