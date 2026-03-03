package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	inventorygrpc "github.com/fjod/go_cart/inventory-service/internal/grpc"
	"github.com/fjod/go_cart/inventory-service/internal/store"
	pb "github.com/fjod/go_cart/inventory-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var initialStock = map[int64]int32{
	1: 100, // Laptop
	2: 500, // Mouse
	3: 300, // Keyboard
	4: 150, // Monitor
	5: 200, // Headphones
}

func main() {
	log := logger.New("inventory-service", "info")
	slog.SetDefault(log)

	port := getEnv("INVENTORY_SERVICE_PORT", "50053")

	memStore := store.NewMemoryStore()

	for productID, quantity := range initialStock {
		if err := memStore.SetStock(productID, quantity); err != nil {
			log.Error("failed to set initial stock", "product_id", productID, "error", err)
			os.Exit(1)
		}
	}
	log.Info("initialized stock", "product_count", len(initialStock))

	server := inventorygrpc.NewInventoryServiceServer(memStore)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	shutdown, err := tracing.InitTracer("inventory-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			logger.UnaryServerInterceptor(log),
		),
	)
	pb.RegisterInventoryServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	go func() {
		log.Info("inventory service listening", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down inventory service")
	grpcServer.GracefulStop()
	if err = memStore.Close(); err != nil {
		log.Error("failed to stop memstore", "error", err)
		os.Exit(1)
	}
	log.Info("inventory service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
