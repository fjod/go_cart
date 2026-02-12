package main

import (
	"context"
	"fmt"
	"log"
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
	log.Println("orders-service starting...")
	var wg sync.WaitGroup

	// Configuration
	grpcPort := getEnv("GRPC_PORT", "50055")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")

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

	// Start Kafka consumer
	kafkaConsumer := consumer.NewConsumer(repo, kafkaBrokers)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		kafkaConsumer.Run(consumerCtx)
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	ordersHandler := ordersgrpc.NewOrdersHandler(repo)
	grpcServer := grpc.NewServer()
	pb.RegisterOrdersServiceServer(grpcServer, ordersHandler)
	reflection.Register(grpcServer)

	go func() {
		log.Printf("Orders service listening on :%s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down orders service...")
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
		log.Println("Consumer stopped cleanly")
	case <-shutdownCtx.Done():
		log.Println("Consumer didn't stop in time")
	}

	kafkaConsumer.Close()
	log.Println("Orders service stopped")
}
