# E-Commerce Platform - Project Status

**Last Updated:** January 16, 2026
**Current Phase:** Phase 1 - Foundation (In Progress)

---

## Overview

This document tracks the implementation status of the e-commerce platform microservices architecture as defined in the [High-Level Implementation Plan](HIGH_LEVEL_IMPLEMENTATION_PLAN.md).

---

## Implementation Status

### Phase 1: Foundation

#### Product Service ✅ Mostly Complete

**Status:** Core functionality implemented, production hardening needed

**Completed:**
- ✅ Go module initialization (`github.com/fjod/go_cart/product-service`)
- ✅ SQLite database driver integration (`modernc.org/sqlite`)
- ✅ Database migration infrastructure using `golang-migrate/migrate`
- ✅ Products table schema creation (product-service/internal/repository/migrations/001_create_products_table.up.sql:1-11)
- ✅ Sample product data seeding with 5 products (product-service/internal/repository/migrations/000002_seed_products.up.sql:1-6)
  - Laptop: $1299.99 (50 in stock)
  - Mouse: $29.99 (200 in stock)
  - Keyboard: $89.99 (100 in stock)
  - Monitor: $399.99 (75 in stock)
  - Headphones: $249.99 (150 in stock)
- ✅ Migration runner implementation (product-service/internal/repository/repository.go:20-46)
- ✅ Domain model (Product entity) (product-service/internal/domain/product.go:1-13)
- ✅ Repository interface pattern for testability (product-service/internal/repository/repository.go:20-24)
- ✅ Repository implementation with context support (product-service/internal/repository/repository.go:61-97)
  - `GetAllProducts(ctx)` - Query all products
  - `Close()` - Resource cleanup
  - `RunMigrations()` - Database schema management
- ✅ Protobuf service definitions (product-service/pkg/proto/product.proto:1-31)
  - Product message with 7 fields
  - GetProductsRequest/Response messages
  - ProductService with GetProducts RPC
- ✅ gRPC service implementation (product-service/internal/grpc/handler.go:1-56)
  - ProductServiceServer implementation
  - GetProducts() handler with error handling
  - Domain to protobuf conversion
- ✅ gRPC server setup (product-service/cmd/main.go:1-49)
  - Server running on port 8084
  - gRPC reflection enabled for debugging
  - Migration execution on startup
- ✅ Unit tests for repository layer (product-service/internal/repository/repository_test.go:1-70)
  - In-memory SQLite testing
  - Context cancellation tests
  - Test coverage for GetAllProducts

**Pending:**
- ⏳ Additional gRPC endpoints
  - ✅ `GetProduct(id)` - Get single product by ID (COMPLETED)
  - `UpdateProduct()` - Update product details
  - `DeleteProduct()` - Delete product
  - `CreateProduct()` - Add new product
- ⏳ Production hardening (see code review issues)
  - Configuration management (environment variables)
  - Graceful shutdown handling
  - Connection pool configuration
  - Structured logging
  - Price precision (use decimal or cents)
  - Timestamp type improvement (use google.protobuf.Timestamp)
- ⏳ Unit tests for gRPC handler layer
- ⏳ Integration tests
- ⏳ Pagination support for GetProducts
- ⏳ Product search/filtering endpoints

**File Structure:**
```
product-service/
├── cmd/
│   └── main.go                          ✅ gRPC server with reflection
├── internal/
│   ├── repository/
│   │   ├── repository.go                ✅ Repository implementation + interface
│   │   ├── repository_test.go           ✅ Unit tests with in-memory DB
│   │   ├── products.db                  ✅ SQLite database
│   │   └── migrations/
│   │       ├── 001_create_products_table.up.sql    ✅
│   │       ├── 001_create_products_table.down.sql  ✅
│   │       ├── 000002_seed_products.up.sql         ✅
│   │       └── 000002_seed_products.down.sql       ✅
│   ├── domain/
│   │   └── product.go                   ✅ Product entity
│   └── grpc/
│       ├── handler.go                   ✅ gRPC service implementation
│       └── handler_test.go              ⏳ Tests pending
├── pkg/
│   └── proto/
│       ├── product.proto                ✅ Protobuf definitions
│       ├── product.pb.go                ✅ Generated code
│       └── product_grpc.pb.go           ✅ Generated gRPC code
├── generate.bat                         ✅ Protobuf generation script (Windows)
└── go.mod                               ✅ Dependencies (gRPC, protobuf added)
```

---

#### Cart Service ✅ Redis Integration Complete

**Status:** All 5 gRPC endpoints with Redis caching layer fully integrated and tested

**Completed:**
- ✅ Go module initialization (`github.com/fjod/go_cart/cart-service`)
- ✅ Domain models (Cart, CartItem) (cart-service/internal/domain/cart.go:1-17)
  - Cart entity with UserID, Items array, timestamps
  - CartItem with ProductID, Quantity, AddedAt
  - BSON tags for MongoDB serialization
- ✅ MongoDB repository interface (cart-service/internal/repository/repository.go:1-18)
  - CartRepository interface with 6 methods
  - GetCart, UpsertCart, AddItem, UpdateItemQuantity, RemoveItem, DeleteCart
- ✅ MongoDB repository implementation (cart-service/internal/repository/mongo_repository.go:1-224)
  - Full CRUD operations for cart management
  - AddItem with upsert logic (creates cart if doesn't exist)
  - Automatic quantity update when same product added
  - TTL index (90 days) for automatic cart cleanup
  - Unique index on user_id
  - Context-aware operations with proper error handling
- ✅ MongoDB connection utility (cart-service/internal/repository/connection.go:1-31)
  - ConnectMongoDB helper with connection pooling
  - Configurable pool sizes (min: 10, max: 100)
  - Connection timeout and server selection timeout
  - Ping verification
- ✅ Repository tests with testcontainers (cart-service/internal/repository/mongodb_repository_test.go:1-179)
  - Integration tests using real MongoDB container (mongo:7)
  - Tests for all CRUD operations
  - Context cancellation tests
  - Test coverage for edge cases (cart not found, item updates, etc.)
- ✅ Dependencies installed
  - go.mongodb.org/mongo-driver v1.17.6
  - github.com/testcontainers/testcontainers-go v0.40.0
  - github.com/testcontainers/testcontainers-go/modules/mongodb v0.40.0
  - github.com/stretchr/testify v1.11.1
- ✅ Complete gRPC service implementation (cart-service/pkg/proto/cart.proto:1-63, cart-service/internal/grpc/handler.go)
  - Protobuf definitions for all 5 endpoints with request/response messages
  - CartService with 5/5 RPC endpoints fully implemented:
    1. AddItem - Add product to cart with validation
    2. GetCart - Retrieve user's cart
    3. UpdateQuantity - Update item quantity (NEW)
    4. RemoveItem - Remove item from cart (NEW)
    5. ClearCart - Clear entire cart (NEW)
  - gRPC handler with product validation via Product Service
  - Server running on port 50052 with reflection support
  - Shared CartResponse message type for consistency
  - Helper function `convertCart()` for domain-to-protobuf conversion with proper timestamp formatting
  - Business rule enforcement (quantity 1-99, product_id validation)
  - **End-to-end tested:** All endpoints successfully interact with MongoDB cartdb collection
- ✅ Comprehensive gRPC handler unit tests (cart-service/internal/grpc/handler_test.go)
  - **10 top-level test functions (16 total test cases including subtests)**
  - TestGetCart_Success - validates cart retrieval with multiple items
  - TestAddItem_Success - validates item addition
  - TestAddItem_NotFound - validates product not found error handling
  - TestAddItem_NoStock - validates out-of-stock error handling
  - TestUpdateQuantity_Success - validates quantity updates (NEW)
  - TestUpdateQuantity_InvalidInput - validates input validation with 4 subtests (NEW)
  - TestRemoveItem_Success - validates item removal (NEW)
  - TestRemoveItem_InvalidInput - validates input validation with 2 subtests (NEW)
  - TestClearCart_Success - validates cart clearing (NEW)
  - TestClearCart_InvalidInput - validates user_id validation (NEW)
  - Mock implementations for Repository and ProductServiceClient
  - **All tests passing (10/10 functions, 16/16 cases)**
- ✅ Product Service integration
  - gRPC client connection to Product Service (localhost:50051)
  - Product validation before adding to cart
- ✅ Environment variable configuration
  - CART_SERVICE_PORT (default: 50052)
  - PRODUCT_SERVICE_ADDR (default: localhost:50051)
  - MONGO_URI (default: mongodb://localhost:27017)
  - MONGO_DB_NAME (default: cartdb)
- ✅ Graceful shutdown handling
- ✅ Protobuf generation script (genProto.bat)
  - Windows batch script for regenerating protobuf code
  - Generates both .pb.go and _grpc.pb.go files

**Redis Caching Layer - ✅ COMPLETE (Steps 1-7 of 7):**
- ✅ **Step 1: Cache interface and Redis implementation** (cart-service/internal/cache/)
  - cache.go - CartCache interface with Get/Set/Delete methods and ErrCacheMiss sentinel
  - redis.go - RedisCache implementation using github.com/redis/go-redis/v9
  - Key format: `cart:{userID}`
  - Base TTL: 15 minutes + random jitter (0-5 minutes) to prevent thundering herd
  - JSON serialization for cart data
- ✅ **Cache unit tests** (cart-service/internal/cache/redis_test.go)
  - 8 test cases using miniredis (in-memory Redis for testing)
  - **All tests passing (8/8)**
- ✅ **Step 2-3: Service Layer created** (cart-service/internal/service/cart_service.go)
  - CartService struct with repository + cache dependencies
  - All 5 methods implemented: GetCart, AddItem, UpdateQuantity, RemoveItem, ClearCart
  - gRPC handlers refactored to use service layer instead of repository directly
- ✅ **Step 5: Redis integrated into Service Layer**
  - Cache-aside pattern with singleflight for GetCart (prevents cache stampede)
  - Write-through invalidation on all mutating operations (async goroutines)
  - Graceful degradation: cache errors logged but don't fail operations
  - Empty cart handling: returns empty cart instead of error for new users
- ✅ **Step 6: Redis configuration in cmd/main.go**
  - REDIS_ADDR environment variable (default: localhost:6379)
  - REDIS_PASSWORD environment variable (default: empty)
  - Redis client wired into service layer
  - Redis ping verification on startup with logging
- ✅ **Service layer unit tests** (cart-service/internal/service/cart_service_test.go)
  - **12 test functions covering all 5 methods:**
    * TestGetCart_Success - cache miss → repo fetch → cache populated
    * TestGetCart_RepoError - database error propagation
    * TestGetCart_CacheHit - returns from cache without hitting repo
    * TestGetCart_CartNotFound_ReturnsEmptyCart - empty cart for new users
    * TestAddItem_Success - adds item and invalidates cache
    * TestAddItem_RepoError - database error propagation
    * TestUpdateQuantity_Success - updates quantity and invalidates cache
    * TestUpdateQuantity_RepoError - database error propagation
    * TestRemoveItem_Success - removes item and invalidates cache
    * TestRemoveItem_RepoError - database error propagation
    * TestClearCart_Success - clears cart and invalidates cache
    * TestClearCart_RepoError - database error propagation
  - Mock implementations for repository and cache with mutex protection
  - Async cache invalidation verified with require.Eventually()
  - **All tests passing (12/12)**
- ✅ **Dependencies installed**
  - github.com/redis/go-redis/v9 - Redis client
  - github.com/alicebob/miniredis/v2 v2.35.0 - In-memory Redis for testing
  - golang.org/x/sync/singleflight - Cache stampede prevention
- ✅ **Step 7: Integration tests with real MongoDB + Redis** (cart-service/internal/grpc/handler_integration_test.go)
  - **5 integration test functions using testcontainers:**
    * TestAddItemToCart_Success - validates adding item with real MongoDB + Redis
    * TestGetCart_Integration - validates cart retrieval with multiple items
    * TestUpdateQuantity_Integration - validates quantity updates in real database
    * TestRemoveItem_Integration - validates item removal from real database
    * TestClearCart_Integration - validates cart clearing
  - Uses testcontainers for both MongoDB (mongo:7) and Redis (redis:latest)
  - Fixed setupRedis bug (premature container cleanup)
  - Discovered race condition in async cache invalidation (documented with workaround)
  - **All tests passing (5/5)**

**Pending:**
- ⏳ Kafka consumer for checkout events
- ⏳ Production hardening
  - Structured logging (replace log.Printf with slog or zap)
  - Request validation improvements
  - Error handling enhancements
- ⏳ Fix async cache invalidation race condition (sync invalidation or read-your-writes pattern)

**File Structure:**
```
cart-service/
├── cmd/
│   └── main.go                          ✅ gRPC server with Redis + service layer wiring
├── internal/
│   ├── domain/
│   │   └── cart.go                      ✅ Cart and CartItem entities
│   ├── cache/
│   │   ├── cache.go                     ✅ CartCache interface
│   │   ├── redis.go                     ✅ Redis implementation with TTL+jitter
│   │   └── redis_test.go                ✅ Unit tests with miniredis (8/8 passing)
│   ├── service/
│   │   ├── cart_service.go              ✅ Service layer with cache-aside pattern
│   │   └── cart_service_test.go         ✅ Unit tests (12/12 passing)
│   ├── grpc/
│   │   ├── handler.go                   ✅ gRPC handlers using service layer
│   │   ├── handler_test.go              ✅ Unit tests (10 functions, 16 cases)
│   │   └── handler_integration_test.go  ✅ Integration tests with testcontainers (5/5 passing)
│   └── repository/
│       ├── repository.go                ✅ Repository interface
│       ├── mongo_repository.go          ✅ MongoDB implementation
│       ├── mongodb_repository_test.go   ✅ Integration tests
│       └── connection.go                ✅ MongoDB connection utility
├── pkg/
│   └── proto/
│       ├── cart.proto                   ✅ Protobuf definitions (5 RPCs complete)
│       ├── cart.pb.go                   ✅ Generated code
│       └── cart_grpc.pb.go              ✅ Generated gRPC code
├── genProto.bat                         ✅ Protobuf generation script
└── go.mod                               ✅ Dependencies configured
```

---

#### API Gateway ✅ Cart & Product Endpoints Complete

**Status:** All 5 cart REST endpoints and 1 product endpoint implemented with comprehensive unit test coverage

**Completed:**
- ✅ Go module initialization (`github.com/fjod/go_cart/api-gateway`)
- ✅ HTTP server setup with go-chi/chi router (api-gateway/cmd/main.go:1-131)
  - Server running on port 8080 (configurable via HTTP_PORT env var)
  - Request timeout: 30 seconds
  - Graceful shutdown handling (10s timeout)
  - SIGINT/SIGTERM signal handling
- ✅ gRPC client connections
  - Cart Service client connection (localhost:50052, configurable via CART_SERVICE_ADDR)
  - Product Service client connection (localhost:50051, configurable via PRODUCT_SERVICE_ADDR)
  - Connection using insecure credentials for development
- ✅ Middleware stack (api-gateway/internal/http/middleware.go:1-39)
  - Logger middleware (chi built-in)
  - Recoverer middleware (panic recovery)
  - RequestID middleware (X-Request-ID header propagation, line 27-38)
  - Timeout middleware (30s default)
  - Compression middleware (level 5)
  - MockAuthMiddleware (simulates JWT authentication, line 11-24)
    - Injects user_id as int64(1) into request context
    - Production-ready placeholder for JWT token validation
- ✅ Complete REST endpoint handlers for cart operations (api-gateway/internal/http/cart_handler.go)
  - CartHandler struct with gRPC client injection
  - **5/5 cart endpoints fully implemented:**
    1. POST /api/v1/cart/items - AddItem endpoint
       - User authentication check via context
       - Request body validation (JSON parsing)
       - Business rule validation (product_id > 0, quantity 1-99)
       - gRPC metadata propagation (user-id, request-id)
       - Comprehensive error handling with proper HTTP status codes
    2. GET /api/v1/cart - GetCart endpoint
       - Retrieves user's shopping cart
       - User authentication check via context
       - gRPC metadata propagation (user-id, request-id)
       - Returns enriched cart data with all items
       - HTTP 200 OK on success
    3. PUT /api/v1/cart/items/{product_id} - UpdateQuantity endpoint (NEW)
       - Updates quantity for specific cart item
       - URL parameter parsing with validation
       - Request body parsing for new quantity
       - Business rule enforcement (quantity 1-99)
    4. DELETE /api/v1/cart/items/{product_id} - RemoveItem endpoint (NEW)
       - Removes specific item from cart
       - URL parameter parsing with validation
       - HTTP 204 No Content on success
    5. DELETE /api/v1/cart - ClearCart endpoint (NEW)
       - Clears entire user cart
       - User authentication required
       - HTTP 204 No Content on success
- ✅ Product REST endpoint handler (api-gateway/internal/http/product_handler.go)
  - ProductHandler struct with gRPC client injection
  - **1/1 product endpoint fully implemented:**
    1. GET /api/v1/products - List all products
       - Calls Product Service via gRPC
       - Maps protobuf response to JSON ProductsResponse
       - Returns array of products with id, name, description, price, image_url
       - HTTP 200 OK on success
- ✅ Product handler unit tests (api-gateway/internal/http/product_handler_test.go)
  - **4 test functions (7 total test cases including subtests)**
  - ProductClientMock implementation for gRPC methods
  - Test coverage:
    * TestGetProducts_Success - validates returning multiple products
    * TestGetProducts_EmptyList - validates empty product list handling
    * TestGetProducts_GRPCErrors - tests 4 gRPC error code mappings
    * TestGetProducts_AllFields - validates all product fields are mapped
  - **All tests passing (4/4 functions, 7/7 cases)**
- ✅ gRPC error mapping to HTTP status codes (api-gateway/internal/http/cart_handler.go:137-178)
  - InvalidArgument → 400 Bad Request
  - NotFound → 404 Not Found
  - AlreadyExists → 409 Conflict
  - Unauthenticated → 401 Unauthorized
  - PermissionDenied → 403 Forbidden
  - ResourceExhausted → 429 Too Many Requests
  - Unavailable → 503 Service Unavailable
  - DeadlineExceeded → 504 Gateway Timeout
  - Default → 500 Internal Server Error
- ✅ Comprehensive unit tests (api-gateway/internal/http/cart_handler_test.go)
  - **17 top-level test functions (38 total test cases including subtests)**
  - ClientMock implementation for all gRPC methods (AddItem, GetCart, UpdateQuantity, RemoveItem, ClearCart)
  - Test coverage for all 5 endpoints:
    * TestGetCart_Success - validates successful cart retrieval
    * TestGetCart_Unauthorized - tests missing authentication
    * TestAddItem_Success - validates successful cart item addition
    * TestAddItem_Unauthorized - tests missing user authentication
    * TestAddItem_InvalidJSON - tests malformed request body handling
    * TestAddItem_InvalidProductID - tests validation with subtests (zero and negative IDs)
    * TestAddItem_InvalidQuantity - tests quantity validation with subtests (zero, negative, >99)
    * TestAddItem_GRPCErrors - tests all 8 gRPC error code mappings with subtests
    * TestUpdateQuantity_Success - validates quantity updates (NEW)
    * TestUpdateQuantity_InvalidProductID - validates URL parameter parsing with 3 subtests (NEW)
    * TestUpdateQuantity_InvalidQuantity - validates quantity rules with 3 subtests (NEW)
    * TestRemoveItem_Success - validates item removal (NEW)
    * TestRemoveItem_InvalidProductID - validates URL parameter parsing with 3 subtests (NEW)
    * TestRemoveItem_Unauthorized - validates authentication (NEW)
    * TestClearCart_Success - validates cart clearing (NEW)
    * TestClearCart_Unauthorized - validates authentication (NEW)
    * TestClearCart_GRPCError - validates error handling (NEW)
  - Uses httptest.NewRecorder() and httptest.NewRequest() for HTTP mocking
  - Demonstrates proper context propagation with user_id and request_id
  - **All tests passing (17/17 functions, 38/38 cases)**
- ✅ Complete routing configuration (api-gateway/cmd/main.go:98-113)
  - GET /api/v1/cart - Get user's cart
  - POST /api/v1/cart/items - Add item to cart
  - PUT /api/v1/cart/items/{product_id} - Update item quantity
  - DELETE /api/v1/cart/items/{product_id} - Remove item
  - DELETE /api/v1/cart - Clear entire cart
  - GET /api/v1/products - List all products
- ✅ Configuration management (api-gateway/cmd/main.go:24-40)
  - Environment variable support for HTTP_PORT and CART_SERVICE_ADDR
  - Config struct with sensible defaults
  - Request timeout, shutdown timeout, max request body size configuration
- ✅ Health check endpoint (api-gateway/cmd/main.go:79-81)
  - GET /health returns {"status": "ok"}
- ✅ Dependencies installed (api-gateway/go.mod:1-17)
  - github.com/go-chi/chi/v5 v5.2.3 (HTTP router)
  - google.golang.org/grpc v1.78.0 (gRPC client)
  - github.com/fjod/go_cart/cart-service (for protobuf definitions)

**Pending:**
- ⏳ Product Service integration (partially complete)
  - ✅ gRPC client connection setup (DONE)
  - ✅ GET /api/v1/products - List products (DONE)
  - ⏳ GET /api/v1/products/{id} - Get product details
- ⏳ Checkout endpoints (future)
  - POST /api/v1/checkout - Initiate checkout
- ⏳ Orders endpoints (future)
  - GET /api/v1/orders - List user's orders
  - GET /api/v1/orders/{id} - Get order details
- ⏳ Real JWT authentication
  - Replace MockAuthMiddleware with actual JWT validation
  - Token parsing and claims extraction
  - Public key/secret configuration
- ⏳ Rate limiting middleware
- ⏳ Circuit breaker implementation
- ⏳ Integration tests with real services
- ⏳ TLS/SSL configuration for production

**File Structure:**
```
api-gateway/
├── cmd/
│   └── main.go                          ✅ HTTP server with chi router, 6 routes active (5 cart + 1 product)
├── internal/
│   └── http/
│       ├── cart_handler.go              ✅ Complete cart handlers (5 endpoints)
│       ├── cart_handler_test.go         ✅ Comprehensive unit tests (17 functions, 38 cases)
│       ├── product_handler.go           ✅ Product handler (1 endpoint)
│       ├── product_handler_test.go      ✅ Unit tests (4 functions, 7 cases)
│       └── middleware.go                ✅ Auth and RequestID middlewares
├── go.mod                               ✅ Dependencies configured
└── go.sum                               ✅ Auto-generated

---

### Phase 2: Checkout Orchestration ❌ Not Started

**Services:**
- ⏳ Checkout Service (saga orchestrator)
- ⏳ Inventory Service (in-memory stub)
- ⏳ Payment Service (mock stub)

---

### Phase 3: Order Processing ❌ Not Started

**Services:**
- ⏳ Orders Service (Kafka consumer)

---

### Phase 4: Integration & Polish ❌ Not Started

**Tasks:**
- ⏳ End-to-end service integration
- ⏳ Distributed tracing
- ⏳ Observability and logging
- ⏳ Testing suite

---

## Infrastructure Status

### Docker Compose Environment ⚡ Partially Set Up

**Completed:**
- ✅ MongoDB container configured (deployments/docker-compose.dev.yml:4-11)
  - mongo:7 image
  - Port mapping: 27017:27017
  - Database name: ecommerce
  - Persistent volume: mongo_data
- ✅ Redis container configured (deployments/docker-compose.dev.yml:13-17)
  - redis:7-alpine image
  - Port mapping: 6379:6379
  - Memory limit: 256mb with LRU eviction policy

**Pending:**
- ⏳ PostgreSQL container
- ⏳ Kafka + Zookeeper containers
- ⏳ Service containers (product-service, cart-service, etc.)

---

## Technology Stack (Actual vs. Planned)

### Databases
- **SQLite Driver:** ✅ Using `modernc.org/sqlite` (pure Go implementation)
  - **Changed from:** `github.com/mattn/go-sqlite3` (CGO-based)
  - **Reason:** Pure Go, no CGO dependencies, easier cross-platform builds
- **MongoDB:** ✅ Configured for Cart Service
  - Docker container (mongo:7) in docker-compose.dev.yml
  - MongoDB driver: go.mongodb.org/mongo-driver v1.17.6
  - Repository implementation with indexes and TTL
- **Redis:** 🔄 Partially Integrated
  - Docker container (redis:7-alpine) in docker-compose.dev.yml
  - Cart Service cache layer complete (cache interface + Redis implementation)
  - Unit tests with miniredis v2.35.0 (8/8 passing)
  - Service layer integration pending
- **PostgreSQL:** ❌ Not configured

### Communication
- **gRPC:** ✅ Product Service implemented (port 8084)
- **Kafka:** ❌ Not configured
- **HTTP/REST:** ❌ Not implemented

### Libraries Installed

**Product Service:**
- ✅ `modernc.org/sqlite` v1.41.0 - SQLite driver
- ✅ `github.com/golang-migrate/migrate/v4` v4.19.1 - Database migrations
- ✅ `github.com/google/uuid` v1.6.0 - UUID generation
- ✅ `google.golang.org/grpc` v1.78.0 - gRPC framework
- ✅ `google.golang.org/protobuf` v1.36.11 - Protocol Buffers

**Cart Service:**
- ✅ `go.mongodb.org/mongo-driver` v1.17.6 - MongoDB driver
- ✅ `github.com/redis/go-redis/v9` - Redis client
- ✅ `github.com/testcontainers/testcontainers-go` v0.40.0 - Integration testing with containers
- ✅ `github.com/testcontainers/testcontainers-go/modules/mongodb` v0.40.0 - MongoDB testcontainer module
- ✅ `github.com/alicebob/miniredis/v2` v2.35.0 - In-memory Redis for testing
- ✅ `github.com/stretchr/testify` v1.11.1 - Testing assertions
- ✅ `google.golang.org/grpc` v1.78.0 - gRPC framework (inherited)
- ✅ `google.golang.org/protobuf` v1.36.11 - Protocol Buffers (inherited)

**API Gateway:**
- ✅ `github.com/go-chi/chi/v5` v5.2.3 - HTTP router and middleware
- ✅ `google.golang.org/grpc` v1.78.0 - gRPC client framework
- ✅ `google.golang.org/protobuf` v1.36.11 - Protocol Buffers (inherited)
- ✅ `github.com/fjod/go_cart/cart-service` - Cart Service protobuf definitions

---

## Next Steps

### Immediate Priorities

1. **✅ Complete Cart Service with Redis Integration - COMPLETED**
   - ✅ Define protobuf messages and service (DONE)
   - ✅ Implement all 5 gRPC handlers (DONE)
   - ✅ Set up gRPC server (DONE)
   - ✅ Add comprehensive unit tests (DONE - 10 functions, 16 test cases)
   - ✅ Create service layer with cache-aside pattern (DONE)
   - ✅ Integrate Redis caching with singleflight (DONE)
   - ✅ Add service layer unit tests (DONE - 12 tests)
   - ✅ Wire Redis into main.go (DONE)
   - ✅ Fix empty cart handling (DONE)
   - ✅ Add integration tests with real Redis + MongoDB (DONE - 5 tests with testcontainers)

2. **✅ Complete API Gateway Cart Endpoints - COMPLETED**
   - ✅ Set up HTTP server with chi router (DONE)
   - ✅ Create gRPC client for Cart Service (DONE)
   - ✅ Implement POST /api/v1/cart/items endpoint (DONE)
   - ✅ Implement GET /api/v1/cart endpoint (DONE)
   - ✅ Implement PUT /api/v1/cart/items/{product_id} endpoint (DONE)
   - ✅ Implement DELETE /api/v1/cart/items/{product_id} endpoint (DONE)
   - ✅ Implement DELETE /api/v1/cart endpoint (DONE)
   - ✅ Add comprehensive unit tests (DONE - 17 functions, 38 test cases, all passing)
   - ✅ Add authentication and request ID middleware (DONE)
   - ⏳ Add integration tests with real Cart Service running (NEXT PRIORITY)
   - ⏳ Replace MockAuthMiddleware with real JWT validation

3. **Add Product Service Integration to API Gateway**
   - ⏳ Create gRPC client for Product Service
   - ⏳ Implement product endpoints:
     - GET /api/v1/products - List all products
     - GET /api/v1/products/{id} - Get product details
   - ⏳ Add unit tests for product handlers

4. **Production Hardening for Product Service** ⚠️
   - Fix critical bug: Remove pointer to interface (handler.go:15, 18)
   - ✅ Add environment variable configuration (DONE)
   - ⏳ Implement graceful shutdown
   - ⏳ Configure database connection pool
   - ⏳ Add structured logging (slog or zap)
   - ⏳ Fix price precision (use cents or decimal)
   - ⏳ Update timestamp to use google.protobuf.Timestamp

5. **Complete Product Service CRUD Operations**
   - ✅ Implement `GetProduct(id)` endpoint (DONE)
   - ⏳ Implement `CreateProduct()` endpoint
   - ⏳ Implement `UpdateProduct()` endpoint
   - ⏳ Implement `DeleteProduct()` endpoint
   - ⏳ Add pagination to `GetProducts()`
   - ⏳ Add unit tests for gRPC handler

6. **Expand Docker Compose Infrastructure**
   - Add PostgreSQL container
   - Add Kafka + Zookeeper containers
   - Add service containers
   - Define service networking

---

## Testing Status

### Product Service
- ✅ Repository unit tests implemented (product-service/internal/repository/repository_test.go)
  - In-memory SQLite testing
  - Context handling tests
  - Context cancellation tests
- ⏳ gRPC handler unit tests pending
- ⏳ Integration tests pending

### Cart Service
- ✅ Repository integration tests implemented (cart-service/internal/repository/mongodb_repository_test.go)
  - Testcontainers with real MongoDB (mongo:7)
  - Full CRUD operation tests
  - Context cancellation tests
  - Edge case coverage (not found, duplicate items, etc.)
- ✅ Cache layer unit tests - COMPLETE (cart-service/internal/cache/redis_test.go)
  - **8 test cases using miniredis (in-memory Redis)**
  - TestGet_Success, TestGet_CacheMiss, TestGet_InvalidJSON
  - TestSet_Success, TestSet_WithTTL (validates 15-20 min jitter)
  - TestDelete_Success, TestDelete_NonExistentKey, TestCacheKey_Format
  - **All tests passing (8/8)**
- ✅ Service layer unit tests - COMPLETE (cart-service/internal/service/cart_service_test.go)
  - **12 test functions covering all 5 service methods**
  - Mock implementations for Repository and Cache with mutex protection
  - Comprehensive coverage:
    * TestGetCart_Success - cache miss → repo fetch → cache populated
    * TestGetCart_RepoError - database error propagation
    * TestGetCart_CacheHit - returns from cache without hitting repo
    * TestGetCart_CartNotFound_ReturnsEmptyCart - empty cart for new users
    * TestAddItem_Success/RepoError - item addition and error handling
    * TestUpdateQuantity_Success/RepoError - quantity update and error handling
    * TestRemoveItem_Success/RepoError - item removal and error handling
    * TestClearCart_Success/RepoError - cart clearing and error handling
  - Async cache invalidation verified with require.Eventually()
  - **All tests passing (12/12)**
- ✅ gRPC handler unit tests - COMPLETE (cart-service/internal/grpc/handler_test.go)
  - **10 top-level test functions, 16 total test cases (including subtests)**
  - Mock implementations for Service and ProductServiceClient
  - Comprehensive coverage for all 5 endpoints
  - **All tests passing (10/10 functions, 16/16 cases)**
- ✅ gRPC handler integration tests - COMPLETE (cart-service/internal/grpc/handler_integration_test.go)
  - **5 integration test functions using real MongoDB + Redis (testcontainers)**
  - TestAddItemToCart_Success - validates full add-to-cart flow
  - TestGetCart_Integration - validates cart retrieval with multiple items
  - TestUpdateQuantity_Integration - validates quantity updates (with race condition workaround)
  - TestRemoveItem_Integration - validates item removal
  - TestClearCart_Integration - validates cart clearing
  - Discovered and documented async cache invalidation race condition
  - **All tests passing (5/5)**

### API Gateway
- ✅ HTTP handler unit tests - COMPLETE (api-gateway/internal/http/cart_handler_test.go)
  - **17 top-level test functions, 38 total test cases (including subtests)**
  - Mock gRPC client implementation (ClientMock) with all 5 methods
  - Comprehensive test coverage for all 5 cart endpoints:
    * TestGetCart_Success - validates successful cart retrieval
    * TestGetCart_Unauthorized - tests missing authentication
    * TestAddItem_Success - validates successful cart item addition
    * TestAddItem_Unauthorized - tests missing user authentication
    * TestAddItem_InvalidJSON - tests malformed request body handling
    * TestAddItem_InvalidProductID - tests validation with 2 subtests
    * TestAddItem_InvalidQuantity - tests quantity validation with 3 subtests
    * TestAddItem_GRPCErrors - tests all 8 gRPC error code mappings with 8 subtests
    * TestUpdateQuantity_Success - validates quantity updates (NEW)
    * TestUpdateQuantity_InvalidProductID - validates URL parsing with 3 subtests (NEW)
    * TestUpdateQuantity_InvalidQuantity - validates quantity rules with 3 subtests (NEW)
    * TestRemoveItem_Success - validates item removal (NEW)
    * TestRemoveItem_InvalidProductID - validates URL parsing with 3 subtests (NEW)
    * TestRemoveItem_Unauthorized - validates authentication (NEW)
    * TestClearCart_Success - validates cart clearing (NEW)
    * TestClearCart_Unauthorized - validates authentication (NEW)
    * TestClearCart_GRPCError - validates error handling (NEW)
  - Uses httptest package for HTTP mocking
  - Context propagation testing (user_id, request_id)
  - **All tests passing (17/17 functions, 38/38 cases)**
- ⏳ Integration tests with real Cart Service pending
- ⏳ End-to-end workflow tests pending

### Overall
- ⏳ E2E tests pending (full flow: add to cart → view cart → checkout)
- ⏳ Load/performance tests pending

---

## Build & Run Status

### Product Service
**Build:** ✅ Compiles successfully (with known interface pointer issue)
**Run:** ✅ Runs gRPC server on port 8084
**Test:** ✅ Repository tests passing

**How to Run:**
```bash
cd product-service
go run cmd/main.go
```

**Expected Output:**
```
2025/12/29 [timestamp] Product-service started
2025/12/29 [timestamp] Migrations completed successfully
2025/12/29 [timestamp] Product service listening on :8084
```

**How to Test:**
```bash
# Run repository tests
cd product-service
go test ./internal/repository/ -v

# Test gRPC endpoint with grpcurl
grpcurl -plaintext localhost:8084 list
grpcurl -plaintext localhost:8084 product.ProductService/GetProducts
```

### Cart Service
**Build:** ✅ Compiles successfully
**Run:** ✅ gRPC server running on port 50052 with all 5 endpoints (AddItem, GetCart, UpdateQuantity, RemoveItem, ClearCart)
**Test:** ✅ Repository integration tests (8 tests), Cache unit tests (8/8), Service unit tests (12/12), gRPC handler unit tests (10 functions, 16 cases), gRPC handler integration tests (5/5)

**How to Run:**
```bash
cd cart-service
go run cmd/main.go
```

**Expected Output:**
```
2026/01/13 [timestamp] Connected to MongoDB at mongodb://localhost:27017
2026/01/13 [timestamp] Connected to product service at localhost:50051
2026/01/13 [timestamp] Cart service listening on port 50052
```

**How to Test:**
```bash
# Run repository integration tests (requires Docker)
cd cart-service
go test ./internal/repository/ -v

# Run cache layer unit tests (using miniredis, no Docker needed)
cd cart-service
go test ./internal/cache/ -v
# Output: 8/8 test cases passing

# Run gRPC handler unit tests
cd cart-service
go test ./internal/grpc/ -v -run "^Test[^I]"
# Output: 10/10 top-level tests passing, 16/16 total test cases

# Run gRPC handler integration tests (requires Docker)
cd cart-service
go test ./internal/grpc/ -v -run "Integration" -timeout 300s
# Output: 5/5 integration tests passing (uses testcontainers for MongoDB + Redis)

# Test all 5 gRPC endpoints with grpcurl
grpcurl -plaintext localhost:50052 list
grpcurl -plaintext -d "{\"user_id\": 1, \"product_id\": 1, \"quantity\": 2}" localhost:50052 cart.CartService/AddItem
grpcurl -plaintext -d "{\"user_id\": 1}" localhost:50052 cart.CartService/GetCart
grpcurl -plaintext -d "{\"user_id\": 1, \"product_id\": 1, \"quantity\": 5}" localhost:50052 cart.CartService/UpdateQuantity
grpcurl -plaintext -d "{\"user_id\": 1, \"product_id\": 1}" localhost:50052 cart.CartService/RemoveItem
grpcurl -plaintext -d "{\"user_id\": 1}" localhost:50052 cart.CartService/ClearCart

# Verify in MongoDB
mongosh cartdb --eval "db.carts.find().pretty()"
```

### API Gateway
**Build:** ✅ Compiles successfully
**Run:** ✅ HTTP server running on port 8080 with all 5 cart REST endpoints
**Test:** ✅ Handler unit tests passing (17/17 functions, 38/38 cases)

**How to Run:**
```bash
cd api-gateway
go run cmd/main.go
```

**Expected Output:**
```
2026/01/13 [timestamp] API Gateway starting on :8080
```

**How to Test:**
```bash
# Run handler unit tests
cd api-gateway
go test ./internal/http/ -v
# Output: 17/17 top-level tests passing, 38/38 total test cases

# Test all 5 REST endpoints with curl (requires Cart Service running on port 50052)
# Add item to cart
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id": 1, "quantity": 2}'

# Get cart
curl -X GET http://localhost:8080/api/v1/cart

# Update item quantity
curl -X PUT http://localhost:8080/api/v1/cart/items/1 \
  -H "Content-Type: application/json" \
  -d '{"quantity": 5}'

# Remove item from cart
curl -X DELETE http://localhost:8080/api/v1/cart/items/1

# Clear entire cart
curl -X DELETE http://localhost:8080/api/v1/cart

# Health check
curl http://localhost:8080/health
```


## Notes

- Using Go 1.25.0
- Project uses Go workspaces (go.work includes product-service, cart-service, and api-gateway)
- Pure Go SQLite driver chosen for better cross-platform compatibility
- Migration files use UTF-8 with BOM encoding
- All services successfully running in parallel:
  - Product Service: localhost:50051 (gRPC) - 2 endpoints (GetProducts, GetProduct)
  - Cart Service: localhost:50052 (gRPC) - **5/5 endpoints complete** (AddItem, GetCart, UpdateQuantity, RemoveItem, ClearCart)
  - API Gateway: localhost:8080 (HTTP/REST) - **6 routes active** (5 cart + 1 product: GET /products)
- Cart Service successfully validated against Product Service and persisting to MongoDB
- API Gateway successfully communicates with Cart Service via gRPC
- **End-to-end testing complete:** All 5 cart operations verified working with live services
- Test pattern established: httptest for HTTP handlers, testcontainers for integration tests, mock implementations for gRPC unit tests
- Service naming consistency achieved: Changed AddCartItemService → CartService (commit d88c94c)
- Protobuf generation automation: Added genProto.bat script for Cart Service
- **Phase 1 cart functionality complete:** Full cart CRUD operations available via REST API with gRPC backend
- **Integration tests added:** Cart Service now has 5 integration tests using testcontainers (MongoDB + Redis)
- **Known issue:** Async cache invalidation race condition discovered during integration testing - cache may serve stale data immediately after mutations (workaround documented, fix pending)

---

## Progress Summary

**Overall Completion:** ~60%

- ✅ Product Service Database Layer: 100%
- ✅ Product Service Domain Layer: 100%
- ✅ Product Service Repository Layer: 100%
- ✅ Product Service gRPC Layer: 80% (GetProducts, GetProduct complete; CRUD pending)
- ✅ Product Service Tests: 50% (Repository done, handler pending)
- ⚠️ Product Service Production Readiness: 60% (env vars added, graceful shutdown needed)
- ✅ Cart Service Database Layer: 100%
- ✅ Cart Service Domain Layer: 100%
- ✅ Cart Service Repository Layer: 100%
- ✅ **Cart Service Service Layer: 100% (cache-aside pattern, singleflight, graceful degradation)**
- ✅ **Cart Service gRPC Layer: 100% (All 5 endpoints using service layer)**
- ✅ **Cart Service Tests: 100% (Repository 8 tests, Cache 8 tests, Service 12 tests, Handler 16 unit + 5 integration = 49 total)**
- ✅ Cart Service Production Readiness: 75% (env vars, graceful shutdown, Redis integration done)
- ✅ **Cart Service Redis Integration: 100% (Steps 1-7/7 complete)**
- ✅ API Gateway HTTP Server: 100% (chi router, graceful shutdown, health check)
- ✅ API Gateway Middleware: 80% (auth mock, request ID done; JWT, rate limiting pending)
- ✅ **API Gateway Cart Endpoints: 100% (All 5 cart endpoints complete with comprehensive unit tests)**
- ✅ **API Gateway Product Endpoints: 50% (GET /products done with tests; GET /products/:id pending)**
- ✅ **API Gateway Tests: 95% (Cart: 17 functions, 38 cases; Product: 4 functions, 7 cases = 21 functions, 45 cases total)**
- ❌ Checkout Service: 0%
- ❌ Orders Service: 0%
- ❌ Inventory Service: 0%
- ❌ Payment Service: 0%
- 🔄 Infrastructure (Docker): 40% (MongoDB and Redis configured, services and Kafka pending)

**Phase 1 Progress:**
- Product Service ~75% complete (core features done, hardening needed)
- **Cart Service ~98% complete (All 5 gRPC endpoints with Redis caching, service layer, unit + integration tests)**
- **API Gateway ~75% complete (All 5 cart + 1 product endpoints complete with tests; e2e tests pending)**
- Docker Infrastructure ~40% complete (MongoDB and Redis done)

**Recent Progress (January 16, 2026):**

**Session 6 - API Gateway Product Endpoint (Current - Uncommitted):**
- ✅ **Added GET /api/v1/products endpoint** (api-gateway/internal/http/product_handler.go)
  - ProductHandler struct with gRPC client injection and timeout
  - Calls Product Service via gRPC GetProducts RPC
  - Maps protobuf response to JSON ProductsResponse
  - Returns products with id, name, description, price, image_url fields
- ✅ **Product handler unit tests** (api-gateway/internal/http/product_handler_test.go)
  - 4 test functions with 7 total test cases
  - ProductClientMock for gRPC client mocking
  - Tests: success, empty list, gRPC errors, all fields validation
  - **All tests passing (4/4 functions, 7/7 cases)**
- ✅ **Updated API Gateway routing** (api-gateway/cmd/main.go)
  - Added Product Service gRPC client connection
  - Added GET /api/v1/products route under /api/v1 route group
  - Fixed chi router duplicate path panic by combining route groups
- ✅ **Updated HIGH_LEVEL_IMPLEMENTATION_PLAN.md**
  - Added product endpoints and DELETE /api/v1/cart to API Gateway endpoint list
- ✅ **Created test-all.ps1 script** for running all tests across workspace modules

**Session 5 - Cart Service Integration Tests:**
- ✅ **Created gRPC handler integration tests** (cart-service/internal/grpc/handler_integration_test.go)
  - Added 5 integration test functions covering all cart operations
  - TestAddItemToCart_Success - validates adding item with real MongoDB + Redis
  - TestGetCart_Integration - validates cart retrieval with multiple items
  - TestUpdateQuantity_Integration - validates quantity updates in real database
  - TestRemoveItem_Integration - validates item removal from real database
  - TestClearCart_Integration - validates cart clearing
- ✅ **Fixed setupRedis bug** - premature container cleanup was terminating Redis before tests ran
- ✅ **Discovered async cache invalidation race condition**
  - Integration tests revealed that async `go invalidateCache()` races with subsequent `GetCart` calls
  - Temporary workaround: 50ms sleep in test (documented as TODO to fix properly)
  - Root cause: cache invalidation is async, but GetCart reads cache immediately after mutation
  - Recommended fix: synchronous cache invalidation or read-your-writes pattern
- ✅ **All 5 integration tests passing**
- ✅ **Redis Integration Step 7/7 complete** - Cart Service caching fully tested with real infrastructure

**Previous Progress (January 15, 2026):**

**Session 4 - Redis Service Layer Integration:**
- ✅ **Created Cart Service service layer** (cart-service/internal/service/cart_service.go)
  - CartService struct with repository + cache + singleflight dependencies
  - GetCart with cache-aside pattern and singleflight for stampede prevention
  - AddItem, UpdateQuantity, RemoveItem, ClearCart with async cache invalidation
  - Empty cart handling: returns empty cart for new users instead of error
  - Graceful degradation: cache failures logged but don't fail operations
  - 1-second timeout on cache invalidation goroutines
- ✅ **Refactored gRPC handlers to use service layer** (cart-service/internal/grpc/handler.go)
  - Changed dependency from repository to service layer
  - All 5 handlers now call service methods instead of repository directly
  - Updated handler tests with mock service layer dependencies
- ✅ **Wired Redis into main.go** (cart-service/cmd/main.go)
  - Redis client initialization with REDIS_ADDR and REDIS_PASSWORD env vars
  - Redis ping verification on startup with "Redis ping succeeded" log
  - Service layer wiring: repo → cache → service → handler
- ✅ **Comprehensive service layer tests** (cart-service/internal/service/cart_service_test.go)
  - 12 test functions covering all 5 service methods
  - Tests for success paths, error paths, cache hits, and empty cart handling
  - Mock repository and cache with mutex protection for thread safety
  - Async cache invalidation verified with require.Eventually()
  - **All tests passing (12/12)**
- ✅ **Fixed empty cart issue**
  - GET /api/v1/cart now returns empty cart `{"user_id":1,"cart":[]}` instead of error
  - Proper handling of repository.ErrCartNotFound in service layer
- ✅ **End-to-end verification with live services**
  - All 5 REST endpoints tested via curl
  - Redis caching working (cache population and invalidation verified)
  - Empty cart behavior confirmed working

**Previous Progress (January 13, 2026):**

**Session 3 - Cart Service & API Gateway Completion:**
- ✅ **Completed all 3 remaining Cart Service gRPC endpoints** (cart-service/internal/grpc/handler.go)
  - UpdateQuantity - Update item quantity with validation (quantity 1-99)
  - RemoveItem - Remove specific item from cart
  - ClearCart - Clear entire user cart
  - All 5/5 endpoints now complete and tested
- ✅ **Expanded Cart Service unit tests to full coverage** (cart-service/internal/grpc/handler_test.go)
  - Added 6 new test functions for the 3 new endpoints
  - TestUpdateQuantity_Success and TestUpdateQuantity_InvalidInput (4 subtests)
  - TestRemoveItem_Success and TestRemoveItem_InvalidInput (2 subtests)
  - TestClearCart_Success and TestClearCart_InvalidInput
  - **Total: 10 top-level test functions, 16 test cases including subtests**
  - All tests passing (10/10 functions, 16/16 cases)
- ✅ **Updated Cart Service protobuf definitions** (cart-service/pkg/proto/cart.proto)
  - Added UpdateQuantityRequest, RemoveItemRequest, ClearCartRequest messages
  - Added 3 new RPC methods to CartService
  - Regenerated protobuf code (cart.pb.go, cart_grpc.pb.go)
- ✅ **Completed all 3 remaining API Gateway cart endpoints** (api-gateway/internal/http/cart_handler.go)
  - PUT /api/v1/cart/items/{product_id} - UpdateQuantity with URL parameter parsing
  - DELETE /api/v1/cart/items/{product_id} - RemoveItem with validation
  - DELETE /api/v1/cart - ClearCart with authentication
  - All 5/5 cart REST endpoints now complete
- ✅ **Expanded API Gateway unit tests to full coverage** (api-gateway/internal/http/cart_handler_test.go)
  - Added 9 new test functions for the 3 new endpoints
  - TestUpdateQuantity_Success, TestUpdateQuantity_InvalidProductID (3 subtests), TestUpdateQuantity_InvalidQuantity (3 subtests)
  - TestRemoveItem_Success, TestRemoveItem_InvalidProductID (3 subtests), TestRemoveItem_Unauthorized
  - TestClearCart_Success, TestClearCart_Unauthorized, TestClearCart_GRPCError
  - Updated ClientMock to include all 5 gRPC methods
  - **Total: 17 top-level test functions, 38 test cases including subtests**
  - All tests passing (17/17 functions, 38/38 cases)
- ✅ **Enabled all cart routes in API Gateway** (api-gateway/cmd/main.go)
  - Uncommented PUT /api/v1/cart/items/{product_id} route
  - Uncommented DELETE /api/v1/cart/items/{product_id} route
  - Added DELETE /api/v1/cart route
  - All 5 cart routes now active
- ✅ **End-to-end verification completed**
  - All 5 Cart Service gRPC endpoints tested with grpcurl
  - All 5 API Gateway REST endpoints tested with running services
  - Verified MongoDB persistence for all operations
  - Confirmed proper error handling and validation across all endpoints
- ✅ **Redis Cache Layer Implementation - Step 1/7 Complete** (cart-service/internal/cache/)
  - Created CartCache interface (cache.go) with Get/Set/Delete methods
  - Implemented RedisCache (redis.go) with TTL+jitter (15-20 min) using github.com/redis/go-redis/v9
  - Comprehensive unit tests (redis_test.go) with 8 test cases using miniredis v2.35.0
  - All tests passing (8/8): cache hit/miss, TTL verification, delete operations, invalid JSON handling
  - Next: Create service layer to integrate Redis with repository

**Session 2 - GetCart Implementation (Commit: fcbe621):**
- ✅ Implemented Cart Service GetCart endpoint (cart-service/internal/grpc/handler.go)
- ✅ Added Cart Service GetCart tests (4 total tests)
- ✅ Updated protobuf definitions with GetCartRequest and CartResponse
- ✅ Implemented API Gateway GET /api/v1/cart endpoint
- ✅ Expanded API Gateway tests to 17 functions
- ✅ Added genProto.bat script for Cart Service

**Session 1 - Initial Cart & API Gateway (Commit: 5564f94, d88c94c):**
- ✅ Fixed Cart Service naming consistency: AddCartItemService → CartService (commit d88c94c)
- ✅ Implemented API Gateway HTTP server with go-chi/chi router
- ✅ Created comprehensive unit tests for API Gateway AddItem handler
- ✅ Implemented middleware stack (auth mock, request ID, timeout, compression)
- ✅ Established gRPC client connection to Cart Service
- ✅ Implemented POST /api/v1/cart/items REST endpoint
- ✅ Created gRPC-to-HTTP error code mapping
- ✅ Added API Gateway to Go workspace
