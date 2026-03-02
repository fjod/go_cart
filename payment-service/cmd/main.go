package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	pg "github.com/fjod/go_cart/payment-service/internal/grpc"
	pb "github.com/fjod/go_cart/payment-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log := logger.New("payment-service", "info")
	slog.SetDefault(log)

	port := getEnv("PAYMENT_SERVICE_PORT", "50054")
	status := pg.RandomStatus{}
	server := pg.NewPaymentServiceServer(status)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	shutdown, err := tracing.InitTracer("payment-service", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterPaymentServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	go func() {
		log.Info("payment service listening", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("failed to serve gRPC", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down payment service")
	grpcServer.GracefulStop()
	log.Info("payment service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
