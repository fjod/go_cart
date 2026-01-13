package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cartpb "github.com/fjod/go_cart/cart-service/pkg/proto"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	h "github.com/fjod/go_cart/api-gateway/internal/http"
)

// import "github.com/fjod/go_cart/cart-service/pkg/proto"

type Config struct {
	HTTPPort           string
	CartServiceAddr    string
	RequestTimeout     time.Duration
	ShutdownTimeout    time.Duration
	MaxRequestBodySize int64
}

func loadConfig() *Config {
	return &Config{
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		CartServiceAddr:    getEnv("CART_SERVICE_ADDR", "localhost:50052"),
		RequestTimeout:     30 * time.Second,
		ShutdownTimeout:    10 * time.Second,
		MaxRequestBodySize: 1 << 20, // 1MB
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
func main() {

	cfg := loadConfig()

	// Set up gRPC connection to Cart Service
	cartServiceConn, err := grpc.NewClient(
		cfg.CartServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to cart service: %v", err)
	}
	defer cartServiceConn.Close()

	cartClient := cartpb.NewCartServiceClient(cartServiceConn)

	cartHandler := h.NewCartHandler(cartClient, cfg.RequestTimeout)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(h.RequestIDMiddleware)
	r.Use(middleware.Timeout(cfg.RequestTimeout))
	r.Use(middleware.Compress(5))
	r.Use(h.MockAuthMiddleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/cart", func(r chi.Router) {
			r.Get("/", cartHandler.GetCart)
			r.Post("/items", cartHandler.AddItem)
			//r.Put("/items/{product_id}", cartHandler.UpdateQuantity)
			//r.Delete("/items/{product_id}", cartHandler.RemoveItem)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("API Gateway starting on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited")
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
