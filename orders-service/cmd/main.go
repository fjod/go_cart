package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fjod/go_cart/orders-service/internal/consumer"
	ordersgrpc "github.com/fjod/go_cart/orders-service/internal/grpc"
	"github.com/fjod/go_cart/orders-service/internal/repository"
	pb "github.com/fjod/go_cart/orders-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
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
	log := logger.New("orders-service", "info")
	slog.SetDefault(log)

	log.Info("orders-service starting")
	var wg sync.WaitGroup

	grpcPort := getEnv("GRPC_PORT", "50055")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "ecommerce")
	migrationsPath := getEnv("MIGRATIONS_PATH", "./internal/repository/migrations")

	port, err := strconv.Atoi(dbPort)
	if err != nil {
		log.Error("invalid DB_PORT", "value", dbPort, "error", err)
		os.Exit(1)
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
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	log.Info("connected to postgres", "host", dbHost, "db", dbName)

	if err := repo.RunMigrations(creds); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	log.Info("database migrations completed")

	kafkaConsumer := consumer.NewConsumer(repo, log, kafkaBrokers)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		kafkaConsumer.Run(consumerCtx)
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	ordersHandler := ordersgrpc.NewOrdersHandler(repo)

	shutdown, err := tracing.InitTracer("orders-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterOrdersServiceServer(grpcServer, ordersHandler)
	reflection.Register(grpcServer)

	go func() {
		log.Info("orders service listening", "port", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down orders service")
	grpcServer.GracefulStop()
	consumerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		log.Info("consumer stopped cleanly")
	case <-shutdownCtx.Done():
		log.Warn("consumer did not stop within timeout")
	}

	kafkaConsumer.Close()
	log.Info("orders service stopped")
}
