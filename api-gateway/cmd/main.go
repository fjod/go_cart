package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	checkoutpb "github.com/fjod/go_cart/checkout-service/pkg/proto"
	orderspb "github.com/fjod/go_cart/orders-service/pkg/proto"
	"github.com/fjod/go_cart/pkg/logger"
	"github.com/fjod/go_cart/pkg/tracing"
	productpb "github.com/fjod/go_cart/product-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	h "github.com/fjod/go_cart/api-gateway/internal/http"
	l "github.com/fjod/go_cart/api-gateway/internal/middleware"
)

type Config struct {
	HTTPPort            string
	CartServiceAddr     string
	ProductServiceAddr  string
	CheckoutServiceAddr string
	OrdersServiceAddr   string
	RequestTimeout      time.Duration
	ShutdownTimeout     time.Duration
	MaxRequestBodySize  int64
}

func loadConfig() *Config {
	return &Config{
		HTTPPort:            getEnv("HTTP_PORT", "8080"),
		CartServiceAddr:     getEnv("CART_SERVICE_ADDR", "localhost:50052"),
		ProductServiceAddr:  getEnv("PRODUCT_SERVICE_ADDR", "localhost:50051"),
		CheckoutServiceAddr: getEnv("CHECKOUT_SERVICE_ADDR", "localhost:50056"),
		OrdersServiceAddr:   getEnv("ORDERS_SERVICE_ADDR", "localhost:50055"),
		RequestTimeout:      30 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		MaxRequestBodySize:  1 << 20, // 1MB
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log := logger.New("api-gateway", "info")
	slog.SetDefault(log)

	shutdown, err := tracing.InitTracer("api-gateway", "localhost:4317")
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())
	cfg := loadConfig()

	cartServiceConn, err := grpc.NewClient(
		cfg.CartServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to cart service", "addr", cfg.CartServiceAddr, "error", err)
		os.Exit(1)
	}
	defer cartServiceConn.Close()

	cartClient := cartpb.NewCartServiceClient(cartServiceConn)
	cartHandler := h.NewCartHandler(cartClient, cfg.RequestTimeout)

	productServiceConn, err := grpc.NewClient(
		cfg.ProductServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to product service", "addr", cfg.ProductServiceAddr, "error", err)
		os.Exit(1)
	}
	defer productServiceConn.Close()

	productClient := productpb.NewProductServiceClient(productServiceConn)
	productHandler := h.NewProductHandler(productClient, cfg.RequestTimeout)

	checkoutServiceConn, err := grpc.NewClient(
		cfg.CheckoutServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to checkout service", "addr", cfg.CheckoutServiceAddr, "error", err)
		os.Exit(1)
	}
	defer checkoutServiceConn.Close()

	checkoutClient := checkoutpb.NewCheckoutServiceClient(checkoutServiceConn)
	checkoutHandler := h.NewCheckoutHandler(checkoutClient, cfg.RequestTimeout)

	ordersServiceConn, err := grpc.NewClient(
		cfg.OrdersServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		log.Error("failed to connect to orders service", "addr", cfg.OrdersServiceAddr, "error", err)
		os.Exit(1)
	}
	defer ordersServiceConn.Close()

	ordersClient := orderspb.NewOrdersServiceClient(ordersServiceConn)
	ordersHandler := h.NewOrdersHandler(ordersClient, cfg.RequestTimeout)

	limiter := l.NewRateLimiter(10, 20) // 10 req/sec, burst of 20
	r := chi.NewRouter()
	r.Use(middleware.RequestID)   // 1. generate request ID
	r.Use(h.RequestIDMiddleware)  // 2. your custom request ID propagation
	r.Use(middleware.Recoverer)   // 3. catch panics
	r.Use(l.MyRequestLogger(log)) // 4. log with request ID + correct status
	r.Use(middleware.Timeout(cfg.RequestTimeout))
	r.Use(middleware.Compress(5))
	r.Use(h.MockAuthMiddleware)
	r.Use(limiter.Middleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/cart", func(r chi.Router) {
			r.Get("/", cartHandler.GetCart)
			r.Post("/items", cartHandler.AddItem)
			r.Put("/items/{product_id}", cartHandler.UpdateQuantity)
			r.Delete("/items/{product_id}", cartHandler.RemoveItem)
			r.Delete("/", cartHandler.ClearCart)
		})

		r.Route("/products", func(r chi.Router) {
			r.Get("/", productHandler.Get)
		})

		r.Post("/checkout", checkoutHandler.InitiateCheckout)

		r.Route("/orders", func(r chi.Router) {
			r.Get("/", ordersHandler.ListOrders)
			r.Get("/{order_id}", ordersHandler.GetOrder)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      otelhttp.NewHandler(r, "api-gateway"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("API gateway starting", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down API gateway")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("API gateway stopped")
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Default().Error("failed to encode response", "error", err)
	}
}
