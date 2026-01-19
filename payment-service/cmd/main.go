package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pg "github.com/fjod/go_cart/payment-service/internal/grpc"
	pb "github.com/fjod/go_cart/payment-service/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := getEnv("PAYMENT_SERVICE_PORT", "50054")
	status := pg.RandomStatus{}
	server := pg.NewPaymentServiceServer(status)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaymentServiceServer(grpcServer, server)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	// Start server in goroutine
	go func() {
		log.Printf("Payment service listening on port %s", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down payment service...")
	grpcServer.GracefulStop()
	log.Println("Payment service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
