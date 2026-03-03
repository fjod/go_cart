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

	checkoutgrpc "github.com/fjod/go_cart/checkout-service/internal/grpc"
	pub "github.com/fjod/go_cart/checkout-service/internal/publisher"
	"github.com/fjod/go_cart/checkout-service/internal/repository"
	"github.com/fjod/go_cart/checkout-service/internal/service"
	pb "github.com/fjod/go_cart/checkout-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

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
	log := logger.New("checkout-service", "info")
	slog.SetDefault(log)

	log.Info("checkout-service starting")
	var wg sync.WaitGroup

	grpcPort := getEnv("GRPC_PORT", "50056")
	cartServiceAddr := getEnv("CART_SERVICE_ADDR", "localhost:50052")
	productServiceAddr := getEnv("PRODUCT_SERVICE_ADDR", "localhost:50051")
	inventoryServiceAddr := getEnv("INVENTORY_SERVICE_ADDR", "localhost:50053")
	paymentServiceAddr := getEnv("PAYMENT_SERVICE_ADDR", "localhost:50054")
	requestTimeout := 5 * time.Second

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

	shutdown, err := tracing.InitTracer("checkout-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())

	kafkaPort := getEnv("KAFKA_PORT", "localhost:9092")
	poller := pub.NewOutboxPoller(repo, log, kafkaPort)
	pollerCtx, pollerCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller.Run(pollerCtx)
	}()

	cartConn, err := grpc.NewClient(cartServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to cart service", "addr", cartServiceAddr, "error", err)
		os.Exit(1)
	}
	defer cartConn.Close()
	cartClient := cartpb.NewCartServiceClient(cartConn)
	log.Info("connected to cart service", "addr", cartServiceAddr)

	productConn, err := grpc.NewClient(productServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to product service", "addr", productServiceAddr, "error", err)
		os.Exit(1)
	}
	defer productConn.Close()
	productClient := productpb.NewProductServiceClient(productConn)
	log.Info("connected to product service", "addr", productServiceAddr)

	inventoryConn, err := grpc.NewClient(inventoryServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to inventory service", "addr", inventoryServiceAddr, "error", err)
		os.Exit(1)
	}
	defer inventoryConn.Close()
	inventoryClient := inventorypb.NewInventoryServiceClient(inventoryConn)
	log.Info("connected to inventory service", "addr", inventoryServiceAddr)

	paymentConn, err := grpc.NewClient(paymentServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to payment service", "addr", paymentServiceAddr, "error", err)
		os.Exit(1)
	}
	defer paymentConn.Close()
	paymentClient := paymentpb.NewPaymentServiceClient(paymentConn)
	log.Info("connected to payment service", "addr", paymentServiceAddr)

	cartHandler := service.NewCartHandler(cartClient, requestTimeout)
	productHandler := service.NewProductHandler(productClient, requestTimeout)
	inventoryHandler := service.NewInventoryHandler(inventoryClient, requestTimeout)
	paymentHandler := service.NewPaymentHandler(paymentClient, requestTimeout)

	checkoutService := service.NewCheckoutService(
		repo,
		cartHandler,
		productHandler,
		inventoryHandler,
		paymentHandler,
		log,
	)

	checkoutServer := checkoutgrpc.NewCheckoutServiceServer(checkoutService)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			logger.UnaryServerInterceptor(log),
		),
	)

	pb.RegisterCheckoutServiceServer(grpcServer, checkoutServer)
	reflection.Register(grpcServer)

	go func() {
		log.Info("checkout service listening", "port", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down checkout service")
	grpcServer.GracefulStop()
	pollerCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		log.Info("poller stopped cleanly")
	case <-shutdownCtx.Done():
		log.Warn("poller did not stop within timeout")
	}

	log.Info("checkout service stopped")
}
