package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	grpcHandler "github.com/fjod/go_cart/product-service/internal/grpc"
	repository "github.com/fjod/go_cart/product-service/internal/repository"
	pb "github.com/fjod/go_cart/product-service/pkg/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	log := logger.New("product-service", "info")
	slog.SetDefault(log)

	dbPath := getEnv("DB_PATH", "./internal/repository/products.db")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./internal/repository/migrations")

	repo, err := repository.NewRepository(dbPath)
	if err != nil {
		log.Error("failed to open repository", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	if err := repo.RunMigrations(migrationsPath); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	log.Info("migrations completed successfully")

	shutdown, err := tracing.InitTracer("product-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))

	productService := grpcHandler.NewProductServiceServer(repo)
	pb.RegisterProductServiceServer(grpcServer, productService)
	reflection.Register(grpcServer)

	port := getEnv("GRPC_PORT", ":50051")
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	log.Info("product service listening", "port", port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Error("failed to serve gRPC", "error", err)
		os.Exit(1)
	}
}
