# E-Commerce Platform - High-Level Implementation Plan

## System Overview

This document outlines the high-level implementation plan for building an e-commerce platform in Go with a microservices architecture. The system supports shopping cart management, checkout processing, and order fulfillment with event-driven architecture.

---

## Architecture Components

### Core Services

1. **API Gateway (BFF - Backend for Frontend)**
2. **Cart Service**
3. **Checkout Service**
4. **Orders Service**
5. **Product Service**
6. **Inventory Service**
7. **Payment Service**

### Infrastructure Components

- **MongoDB** - Document storage for cart data
- **PostgreSQL** - Relational storage for checkouts and orders
- **Redis** - Caching layer for cart data
- **Kafka** - Message broker for event-driven communication
- **gRPC** - Inter-service communication protocol

### Key Architectural Principles

**Service Boundaries:**
- **Product Service** = Catalog data only (name, price, description, image_url)
- **Inventory Service** = Stock levels and reservations (Available, Reserved counts)
- **Separation Rationale**: Different scaling needs, update frequencies, and business logic

**Relationship Between Services:**
- Orders Service creates orders **asynchronously** after Checkout Service publishes events
- Link maintained via `orders.checkout_id → checkout_sessions.id` (not reverse)
- Checkout completes and returns to user **before** order record is created
- This is eventual consistency by design - enables independent service scaling

**Data Ownership:**
- Each service owns its data exclusively
- No cross-service database queries
- All communication via gRPC (sync) or Kafka (async)

---

## Service Descriptions

### 1. API Gateway (BFF)

**Purpose:**
Entry point for all client requests. Routes HTTP/REST requests to appropriate backend microservices via gRPC.

**Responsibilities:**
- HTTP to gRPC translation
- Authentication and authorization
- Rate limiting and throttling
- Request/response transformation
- Circuit breaking for backend services
- API versioning

**Technology Stack:**
- Framework: `go-chi/chi` or Go standard library `net/http`
- Auth: JWT token validation
- Client libraries: gRPC clients for all backend services
- Observability: OpenTelemetry for distributed tracing

**Endpoints:**
```
POST   /api/v1/cart/items              → Add item to cart
GET    /api/v1/cart                    → Get user's cart
PUT    /api/v1/cart/items/:id          → Update item quantity
DELETE /api/v1/cart/items/:id          → Remove item from cart
DELETE /api/v1/cart                    → Clear entire cart
GET    /api/v1/products                → List all products
GET    /api/v1/products/:id            → Get product details
POST   /api/v1/checkout                → Initiate checkout process
GET    /api/v1/orders                  → List user's orders
GET    /api/v1/orders/:id              → Get order details
```

**Configuration:**
- Backend service addresses (cart, checkout, orders, product)
- JWT secret/public key for token validation
- Rate limiting rules
- Timeout configurations

---

### 2. Cart Service

**Purpose:**
Manages user shopping carts with high-performance read/write operations using MongoDB and Redis caching.

**Responsibilities:**
- CRUD operations on shopping carts
- Cart item validation against Product Service
- Cache management for frequently accessed carts
- Cart expiration (90 days inactivity)
- Business rule enforcement (max items, quantity limits)
- Cart clearing on successful checkout (event-driven)

**Technology Stack:**
- Storage: MongoDB (primary storage)
- Cache: Redis (15-minute TTL with jitter)
- Protocol: gRPC for service-to-service, HTTP via gateway
- Events: Kafka consumer for checkout events

**Data Model (MongoDB):**
```json
{
  "_id": "ObjectId",
  "user_id": "string",
  "items": [
    {
      "product_id": "int64",
      "quantity": "int",
      "added_at": "timestamp"
    }
  ],
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

**Business Rules:**
- Maximum 50 items per cart
- Quantity range: 1-99 per item
- Product must exist (validated via Product Service)
- Carts expire after 90 days of inactivity

**gRPC Service Interface:**
```protobuf
service CartService {
  rpc GetCart(GetCartRequest) returns (GetCartResponse);
  rpc AddItem(AddItemRequest) returns (AddItemResponse);
  rpc UpdateQuantity(UpdateQuantityRequest) returns (UpdateQuantityResponse);
  rpc RemoveItem(RemoveItemRequest) returns (RemoveItemResponse);
  rpc ClearCart(ClearCartRequest) returns (ClearCartResponse);
}
```

**Caching Strategy:**
- Lazy loading: Check Redis first, fallback to MongoDB
- Write-through invalidation: Delete cache on updates
- TTL: 15 minutes with random jitter (0-5 minutes) to prevent thundering herd
- Singleflight pattern to prevent cache stampede

**Redis Integration Implementation Plan:**

This step-by-step plan ensures each layer is independently tested before integration:

**Step 1: Create Redis Cache Implementation (1-2 hours)**
- Create `cart-service/internal/cache/cache.go` - Interface definition
    - `Get(ctx, userID)` - Retrieve cart from cache
    - `Set(ctx, userID, cart)` - Store cart with TTL + jitter
    - `Delete(ctx, userID)` - Invalidate cache entry
    - `ErrCacheMiss` - Sentinel error for cache misses
- Create `cart-service/internal/cache/redis.go` - Redis implementation
    - Use `github.com/redis/go-redis/v9` client
    - Key format: `cart:{userID}`
    - Base TTL: 15 minutes + random jitter (0-5 minutes)
    - JSON serialization for cart data
- Create `cart-service/internal/cache/redis_test.go` - Unit tests
    - Use testcontainers with `redis:7-alpine`
    - Test Get/Set/Delete operations
    - Test cache miss handling
    - Test TTL with jitter
- Verification: `go test ./internal/cache/... -v` (all tests pass)

**Step 2: Create Service Layer Without Redis (1 hour)**
- Create `cart-service/internal/service/cart_service.go`
    - `CartService` struct with repository dependency
    - Implement all methods as direct passthroughs to repository:
        - `GetCart(ctx, userID)` → calls `repo.GetCart()`
        - `AddItem(ctx, userID, productID, quantity)` → calls `repo.AddItem()`
        - `UpdateQuantity(ctx, userID, productID, quantity)` → calls `repo.UpdateQuantity()`
        - `RemoveItem(ctx, userID, productID)` → calls `repo.RemoveItem()`
        - `ClearCart(ctx, userID)` → calls `repo.ClearCart()`
- Create `cart-service/internal/service/cart_service_test.go`
    - Mock repository interface
    - Test each method calls repository correctly
    - Test error propagation
- Verification: `go test ./internal/service/... -v` (all tests pass)

**Step 3: Refactor gRPC Handler to Use Service Layer (1-2 hours)**
- Modify `cart-service/internal/grpc/handler.go`
    - Change `CartServiceServer` to use `service.CartService` instead of `repository.CartRepository`
    - Update all handler methods to call service layer:
        - `GetCart` → `s.service.GetCart(ctx, req.UserId)`
        - `AddItem` → `s.service.AddItem(ctx, req.UserId, req.ProductId, req.Quantity)`
        - `UpdateQuantity` → `s.service.UpdateQuantity(...)`
        - `RemoveItem` → `s.service.RemoveItem(...)`
        - `ClearCart` → `s.service.ClearCart(...)`
    - Keep protobuf conversion logic in handlers
- Update `cart-service/internal/grpc/handler_test.go`
    - Change mocks from repository to service layer
    - Verify all 10 test functions still pass
- Modify `cart-service/cmd/main.go`
    - Wire: `repo := repository.New()` → `service := service.New(repo)` → `handler := grpc.New(service)`
- Verification:
    - `go test ./internal/grpc/... -v` (10/10 tests pass)
    - `go run cmd/main.go` (service starts)
    - Test with grpcurl (functionality unchanged)

**Step 4: Add Redis to Docker Compose (15 minutes)**
- Create/update `deployments/docker-compose.yml`
    - Add Redis service with `redis:7-alpine` image
    - Port: 6379
    - MaxMemory: 256MB with `allkeys-lru` eviction policy
- Verification:
    - `docker-compose up -d redis`
    - `docker-compose ps` (redis running)
    - `redis-cli ping` (returns PONG)

**Step 5: Integrate Redis into Service Layer (2 hours)**
- Modify `cart-service/internal/service/cart_service.go`
    - Add `cache cache.CartCache` field to `CartService`
    - Add `sfg singleflight.Group` field for cache stampede prevention
    - Add `golang.org/x/sync/singleflight` dependency
    - Update `NewCartService()` to accept cache parameter
    - Implement cache-aside pattern in `GetCart()`:
        1. Use singleflight to group concurrent requests
        2. Check cache first (`cache.Get()`)
        3. On cache miss, fetch from repository
        4. Populate cache asynchronously (fire-and-forget)
        5. Return cart data
    - Implement write-through invalidation in mutating methods:
        - `AddItem()`: After repo success → `cache.Delete()`
        - `UpdateQuantity()`: After repo success → `cache.Delete()`
        - `RemoveItem()`: After repo success → `cache.Delete()`
        - `ClearCart()`: After repo success → `cache.Delete()`
    - Log cache errors but don't fail operations (graceful degradation)
- Update `cart-service/internal/service/cart_service_test.go`
    - Add tests for cache hit scenarios
    - Add tests for cache miss scenarios
    - Add tests for cache invalidation on writes
    - Mock both repository and cache interfaces
- Modify `cart-service/cmd/main.go`
    - Initialize Redis client with `redis.NewClient()`
    - Wire cache into service: `cache := cache.NewRedis(redisClient)` → `service := service.New(repo, cache)`
- Verification:
    - `go test ./internal/service/... -v` (all cache tests pass)
    - `REDIS_ADDR=localhost:6379 go run cmd/main.go`
    - Test cache behavior:
        - `grpcurl ... GetCart` → `redis-cli KEYS "cart:*"` (cache populated)
        - `grpcurl ... GetCart` again (should serve from cache - add logging to verify)
        - `grpcurl ... AddItem` → `redis-cli KEYS "cart:*"` (cache invalidated)

**Step 6: Add Configuration Management (30 minutes)**
- Modify `cart-service/cmd/main.go`
    - Create `Config` struct with fields:
        - `MongoURI` (env: MONGODB_URI, default: mongodb://localhost:27017)
        - `MongoDBName` (env: MONGODB_DATABASE, default: ecommerce)
        - `RedisAddr` (env: REDIS_ADDR, default: localhost:6379)
        - `RedisPassword` (env: REDIS_PASSWORD, default: empty)
        - `GRPCPort` (env: GRPC_PORT, default: 50052)
    - Implement `loadConfig()` function with `os.Getenv()` and defaults
    - Use config values for all connections
- Verification:
    - Test with environment variables: `REDIS_ADDR=localhost:6379 MONGODB_URI=mongodb://localhost:27017 go run cmd/main.go`
    - Service starts successfully with custom config

**Step 7: Integration Testing (1 hour)**
- Create `cart-service/internal/service/integration_test.go` (optional)
    - Use testcontainers for both MongoDB and Redis
    - Test full workflow: GetCart (miss) → populate cache → GetCart (hit) → AddItem (invalidate) → GetCart (miss) → repopulate
- Manual end-to-end verification:
    - Start infrastructure: `docker-compose up -d`
    - Start service: `go run cmd/main.go`
    - Test workflow with grpcurl + redis-cli verification
- Verification: Complete cart workflow works with caching

**Implementation Notes:**
- Each step is independently verifiable before moving to the next
- Service layer works without Redis (Step 3), then caching is added (Step 5)
- Cache failures degrade gracefully - service continues working if Redis is down
- Total implementation time: 7-9 hours focused work
- This plan prevents "big bang" integration issues by testing each layer in isolation

**Kafka Events (Consumer):**
- Topic: `checkout-events`
- Event: `CheckoutCompleted`
- Action: Clear user's cart after successful checkout

---

### 3. Checkout Service

**Purpose:**
Orchestrates the checkout process as a saga coordinator, managing distributed transactions across inventory, payment, and order creation.

**Responsibilities:**
- Create and manage checkout sessions
- Saga orchestration (inventory reservation → payment → order creation)
- Compensation logic for failures
- Idempotency handling (prevent duplicate charges)
- Transactional outbox pattern for event publishing
- State machine management for checkout lifecycle

**Technology Stack:**
- Storage: PostgreSQL (ACID transactions required)
- Events: Kafka producer (via outbox pattern)
- Protocol: gRPC for inventory/payment calls
- Clients: gRPC clients for Inventory and Payment services

**Data Model (PostgreSQL):**

**checkout_sessions table:**
```sql
CREATE TABLE checkout_sessions (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    cart_snapshot JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    
    -- Saga state tracking
    inventory_reservation_id VARCHAR(255),
    payment_id VARCHAR(255),
    -- Note: order_id is NOT stored here because Orders Service creates the order
    -- asynchronously after consuming the Kafka event. The link is maintained via
    -- orders.checkout_id → checkout_sessions.id
    
    -- Metadata
    total_amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    shipping_address JSONB,
    payment_method VARCHAR(50),
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX idx_checkout_idempotency ON checkout_sessions(idempotency_key);
CREATE INDEX idx_checkout_user ON checkout_sessions(user_id);
CREATE INDEX idx_checkout_status ON checkout_sessions(status);
```

**outbox_events table:**
```sql
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP
);

CREATE INDEX idx_outbox_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;

ALTER TABLE outbox_events 
ADD CONSTRAINT fk_outbox_checkout 
FOREIGN KEY (aggregate_id) 
REFERENCES checkout_sessions(id);
```

**Notes:**
- `cart_snapshot` in checkout_sessions: Minimal cart data at checkout time (product_id, quantity, price)
- `payload` in outbox_events: Enriched event data for Kafka consumers (includes checkout_id, user_id, items with names, timestamps, etc.)
- Both are needed: cart_snapshot for saga compensation/audit, payload for event consumers

**Checkout State Machine:**
```
INITIATED
  ↓
INVENTORY_RESERVED
  ↓
PAYMENT_PENDING
  ↓
PAYMENT_COMPLETED
  ↓
COMPLETED

(Any step can transition to FAILED with compensation)
```

**Saga Flow (Single Row, Multiple Updates):**

One checkout session = one database row that gets updated through state transitions.

1. **Create Session**: Generate checkout session with idempotency key
   ```sql
   INSERT INTO checkout_sessions (id, user_id, cart_snapshot, status, idempotency_key, total_amount)
   VALUES ('uuid', 'user-123', '[...]', 'INITIATED', 'idem-key', 2599.98);
   ```

2. **Reserve Inventory**: Call Inventory Service (sync, 30s timeout)
   - On success:
     ```sql
     UPDATE checkout_sessions 
     SET status = 'INVENTORY_RESERVED', 
         inventory_reservation_id = 'res-456',
         updated_at = NOW()
     WHERE id = 'uuid';
     ```
   - On failure: Mark FAILED, return error to user

3. **Process Payment**: Call Payment Service (sync, 60s timeout)
   - On success:
     ```sql
     UPDATE checkout_sessions 
     SET status = 'PAYMENT_COMPLETED', 
         payment_id = 'pay-789',
         updated_at = NOW()
     WHERE id = 'uuid';
     ```
   - On failure: Compensate (release inventory), mark FAILED

4. **Publish Event & Complete**: Write to outbox + update status (atomic transaction)
   ```sql
   BEGIN;
   
   INSERT INTO outbox_events (aggregate_id, event_type, payload)
   VALUES ('uuid', 'CheckoutCompleted', '{...enriched event data...}');
   
   UPDATE checkout_sessions 
   SET status = 'COMPLETED',
       completed_at = NOW(),
       updated_at = NOW()
   WHERE id = 'uuid';
   
   COMMIT;
   ```

5. **Outbox Poller**: Background job publishes events to Kafka (eventual)

6. **Return to User**: Checkout completes, user receives response immediately

**Important Notes:**
- **Idempotency**: Check `idempotency_key` BEFORE starting saga - return existing result if found
- **State Persistence**: Each saga step updates the SAME row, never creates new rows
- **Atomic Writes**: Step 4 uses transaction to ensure outbox event + status update are atomic
- **Async Processing**: Orders Service and Cart Service process events AFTER checkout returns

**Saga Flow:**
1. **Create Session**: Generate checkout session with idempotency key
2. **Reserve Inventory**: Call Inventory Service (sync, 30s timeout)
    - On failure: Mark FAILED, return error
3. **Process Payment**: Call Payment Service (sync, 60s timeout)
    - On failure: Release inventory reservation, mark FAILED
4. **Publish Event**: Write to outbox table (transactional)
5. **Outbox Poller**: Background job publishes events to Kafka
6. **Mark Complete**: Update session status to COMPLETED

**gRPC Service Interface:**
```protobuf
service CheckoutService {
  rpc InitiateCheckout(InitiateCheckoutRequest) returns (InitiateCheckoutResponse);
  rpc GetCheckoutStatus(GetCheckoutStatusRequest) returns (GetCheckoutStatusResponse);
}
```

**Kafka Events (Producer):**
- Topic: `checkout-events`
- Event: `CheckoutCompleted`
- Payload: checkout_id, user_id, items, total_amount

**Outbox Poller:**
- Polls `outbox_events` table every 1 second
- Fetches unprocessed events (processed_at IS NULL)
- Publishes to Kafka
- Marks as processed on success
- Retries on failure (idempotent)

**Idempotency:**
- Client provides idempotency_key in checkout request
- Check if session with same key exists before processing
- Return existing result if already completed/failed

---

### 4. Orders Service

**Purpose:**
Manages order lifecycle by consuming checkout events and providing order query capabilities.

**Responsibilities:**
- Consume checkout completion events from Kafka
- Create order records in database
- Track order status (confirmed → processing → shipped → delivered)
- Provide order query API
- Ensure idempotent event processing
- Order history management

**Technology Stack:**
- Storage: PostgreSQL
- Events: Kafka consumer
- Protocol: gRPC for queries

**Data Model (PostgreSQL):**

**orders table:**
```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    checkout_id UUID NOT NULL UNIQUE,
    user_id VARCHAR(255) NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**order_items table:**
```sql
CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id),
    product_id BIGINT NOT NULL,
    quantity INT NOT NULL,
    price_snapshot DECIMAL(10, 2) NOT NULL,
    product_name VARCHAR(255) NOT NULL
);
```

**Order Status Flow:**
```
CONFIRMED → PROCESSING → SHIPPED → DELIVERED
            ↓
         CANCELLED (optional)
```

**Event Processing:**
1. Consume `CheckoutCompleted` event from Kafka
2. Check if order with checkout_id already exists (idempotency)
3. Create order record with items
4. Insert into `orders` and `order_items` tables (transaction)
5. Commit Kafka offset

**gRPC Service Interface:**
```protobuf
service OrdersService {
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc UpdateOrderStatus(UpdateOrderStatusRequest) returns (UpdateOrderStatusResponse);
}
```

**Kafka Events (Consumer):**
- Topic: `checkout-events`
- Event: `CheckoutCompleted`
- Consumer Group: `orders-service`
- Action: Create order from checkout data

---

### 5. Product Service

**Purpose:**
Provides product catalog information. This is a stub service for learning purposes.

**Responsibilities:**
- Store and retrieve product information
- Batch product queries (for cart enrichment)
- Product existence validation
- Product catalog management

**Technology Stack:**
- Storage: SQLite (sufficient for learning/testing)
- Protocol: gRPC

**Data Model (SQLite):**
```sql
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price REAL NOT NULL,
    image_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

> **Note:** Stock/inventory data is managed by the Inventory Service, not the Product Service. This separation allows independent scaling and follows the single-responsibility principle.

**gRPC Service Interface:**
```protobuf
service ProductService {
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc GetProducts(GetProductsRequest) returns (GetProductsResponse);
  rpc ValidateProducts(ValidateProductsRequest) returns (ValidateProductsResponse);
}
```

**Sample Data:**
- Laptop: $1299.99
- Mouse: $29.99
- Keyboard: $89.99
- Monitor: $399.99
- Headphones: $249.99

---

### 6. Inventory Service

**Purpose:**
Manages product stock levels and inventory reservations during checkout. This is a stub/mock service for learning purposes that holds stock data in-memory.

**Responsibilities:**
- **Store and query product stock levels** (in-memory)
- Reserve items temporarily during checkout (5-minute TTL)
- Confirm reservation (permanent stock deduction)
- Release reservation (timeout or payment failure)
- Automatic expiration of unreserved items
- Validate stock availability for cart operations

**Technology Stack:**
- Storage: In-memory map (no persistence for stub)
- Protocol: gRPC

**Data Structures:**
```go
type InventoryStore struct {
    mu           sync.RWMutex
    items        map[int64]*InventoryItem
    reservations map[string]*Reservation
}

type InventoryItem struct {
    ProductID int64
    Available int
    Reserved  int
}

type Reservation struct {
    ID         string
    CheckoutID string
    Items      []ReservationItem
    Status     ReservationStatus // RESERVED, CONFIRMED, RELEASED, EXPIRED
    ExpiresAt  time.Time
    CreatedAt  time.Time
}

type ReservationItem struct {
    ProductID int64
    Quantity  int
}
```

**Initial Stock Data:**
Initialize with the same 5 products as Product Service:
```go
func NewInventoryStore() *InventoryStore {
    return &InventoryStore{
        items: map[int64]*InventoryItem{
            1: {ProductID: 1, Available: 50, Reserved: 0},   // Laptop
            2: {ProductID: 2, Available: 200, Reserved: 0},  // Mouse
            3: {ProductID: 3, Available: 100, Reserved: 0},  // Keyboard
            4: {ProductID: 4, Available: 75, Reserved: 0},   // Monitor
            5: {ProductID: 5, Available: 150, Reserved: 0},  // Headphones
        },
        reservations: make(map[string]*Reservation),
    }
}
```

**gRPC Service Interface:**
```protobuf
service InventoryService {
  rpc GetStock(GetStockRequest) returns (GetStockResponse);        // Query stock for product(s)
  rpc Reserve(ReserveRequest) returns (ReserveResponse);           // Reserve during checkout
  rpc Confirm(ConfirmRequest) returns (ConfirmResponse);           // Confirm after payment
  rpc Release(ReleaseRequest) returns (ReleaseResponse);           // Release on failure
}
```

**Reservation Flow:**
1. **Reserve**: Create reservation with 5-minute TTL, start expiration timer
2. **Confirm**: Mark as confirmed (called after successful payment)
3. **Release**: Delete reservation (called on payment failure)
4. **Auto-Expire**: Background timer releases reservation after 5 minutes if not confirmed

---

### 7. Payment Service

**Purpose:**
Simulates external payment gateway for processing payments. This is a stub/mock service for learning purposes.

**Responsibilities:**
- Process payment charges
- Simulate payment processing delays (1-2 seconds)
- Return success/failure based on configurable success rate
- Process refunds

**Technology Stack:**
- Storage: In-memory (no persistence for stub)
- Protocol: gRPC

**Mock Behavior:**
- Processing delay: 1-2 seconds (simulates network/gateway latency)
- Success rate: 95% (configurable)
- Failure reasons: insufficient funds, card declined, expired card, invalid CVV, network error

**gRPC Service Interface:**
```protobuf
service PaymentService {
  rpc Charge(ChargeRequest) returns (ChargeResponse);
  rpc Refund(RefundRequest) returns (RefundResponse);
}
```

**Charge Response:**
```go
type ChargeResponse struct {
    PaymentID     string // UUID
    Status        string // "SUCCESS" or "FAILED"
    TransactionID string // "TXN-{timestamp}"
    Message       string // Success/failure message
}
```

---

## System Integration Flow

### User Add Item to Cart
```
User → API Gateway → Cart Service → Product Service (validation)
                   ↓
                MongoDB (persist)
                   ↓
                Redis (invalidate cache)
```

### User View Cart
```
User → API Gateway → Cart Service
                   ↓
                Redis (check cache)
                   ↓ (miss)
                MongoDB (fetch cart)
                   ↓
                Product Service (enrich with product details)
                   ↓
                Redis (populate cache)
                   ↓
                User (return enriched cart)
```

### Checkout Process
```
User → API Gateway → Checkout Service
                   ↓
                Create checkout session (PostgreSQL)
                   ↓
                Inventory Service (reserve items - sync)
                   ↓
                Payment Service (charge - sync)
                   ↓
                Write to outbox table (PostgreSQL transaction)
                   ↓
                Outbox Poller → Kafka (publish CheckoutCompleted event)
                   ↓
                   ├─→ Orders Service (create order)
                   └─→ Cart Service (clear cart)
```

---

## Technology Stack Summary

### Programming Language
- **Go 1.21+** for all services

### Storage Systems
- **MongoDB 7.0**: Cart data (document-oriented)
- **PostgreSQL 16**: Checkouts, orders, outbox (relational with ACID)
- **Redis 7**: Cart caching (in-memory)
- **SQLite 3**: Product catalog (learning/testing)

### Communication
- **gRPC**: Inter-service synchronous communication
- **Kafka**: Asynchronous event-driven communication
- **HTTP/REST**: Client-facing API (via gateway)

### Libraries & Frameworks

**Core:**
- `google.golang.org/grpc` - gRPC framework
- `google.golang.org/protobuf` - Protocol buffers
- `github.com/go-chi/chi/v5` - HTTP router (API Gateway)

**Storage:**
- `go.mongodb.org/mongo-driver` - MongoDB driver
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client
- `modernc.org/sqlite` - SQLite driver (pure Go, no CGO)

**Messaging:**
- `github.com/segmentio/kafka-go` - Kafka client

**Utilities:**
- `github.com/google/uuid` - UUID generation
- `golang.org/x/sync/singleflight` - Cache stampede prevention

**Observability:**
- `go.opentelemetry.io/otel` - Distributed tracing
- `github.com/rs/zerolog` or `go.uber.org/zap` - Structured logging

---

## Project Structure

```
ecommerce-platform/
├── api-gateway/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handler/          # HTTP handlers
│   │   ├── middleware/       # Auth, rate limiting, tracing
│   │   ├── client/           # gRPC clients
│   │   └── config/
│   └── go.mod
│
├── cart-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   ├── repository/       # MongoDB implementation
│   │   ├── cache/            # Redis implementation
│   │   ├── service/          # Business logic
│   │   ├── grpc/             # gRPC handlers
│   │   └── consumer/         # Kafka consumer
│   ├── pkg/proto/            # Protobuf definitions
│   └── go.mod
│
├── checkout-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   ├── repository/       # PostgreSQL implementation
│   │   ├── saga/             # Saga orchestration
│   │   ├── client/           # gRPC clients
│   │   ├── publisher/        # Outbox poller
│   │   └── grpc/             # gRPC handlers
│   ├── pkg/proto/
│   └── go.mod
│
├── orders-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   ├── repository/       # PostgreSQL implementation
│   │   ├── consumer/         # Kafka consumer
│   │   └── grpc/             # gRPC handlers
│   ├── pkg/proto/
│   └── go.mod
│
├── product-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   ├── repository/       # SQLite implementation
│   │   └── grpc/             # gRPC handlers
│   ├── pkg/proto/
│   └── go.mod
│
├── inventory-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   ├── store/            # In-memory store
│   │   └── grpc/             # gRPC handlers
│   ├── pkg/proto/
│   └── go.mod
│
├── payment-service/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── domain/           # Entities
│   │   └── grpc/             # gRPC handlers
│   ├── pkg/proto/
│   └── go.mod
│
├── pkg/                      # Shared packages
│   ├── logger/               # Structured logging
│   ├── tracing/              # OpenTelemetry
│   ├── kafka/                # Kafka helpers
│   └── errors/               # Common errors
│
├── deployments/
│   ├── docker-compose.yml    # Local dev environment
│   └── kubernetes/           # K8s manifests (future)
│
├── scripts/
│   ├── setup-dev.sh          # Initialize dev env
│   ├── seed-data.sh          # Populate test data
│   └── test-e2e.sh           # E2E testing
│
└── README.md
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
**Services to Build:**
- Cart Service (MongoDB + Redis)
- Product Service (SQLite stub)
- API Gateway (basic routing)

**Infrastructure:**
- Docker Compose with MongoDB, Redis
- Protobuf definitions for Cart and Product services
- Basic HTTP endpoints in gateway

**Deliverable:**
Users can add/view/edit cart items via REST API

---

### Phase 2: Checkout Orchestration (Week 3-4)
**Services to Build:**
- Checkout Service (saga orchestrator)
- Inventory Service (in-memory stub)
- Payment Service (mock stub)

**Infrastructure:**
- Add PostgreSQL to Docker Compose
- Add Kafka + Zookeeper to Docker Compose
- Implement outbox pattern in Checkout Service

**Deliverable:**
Users can complete checkout with inventory reservation and payment processing

---

### Phase 3: Order Processing (Week 5)
**Services to Build:**
- Orders Service (Kafka consumer)

**Infrastructure:**
- Kafka consumer groups
- Order database schema

**Deliverable:**
Orders created from successful checkouts, viewable via API

---

### Phase 4: Integration & Polish (Week 6)
**Tasks:**
- Connect Cart Service to Kafka (clear cart on checkout)
- Add distributed tracing (OpenTelemetry)
- Implement comprehensive error handling
- Add observability (metrics, logging)
- End-to-end testing

**Deliverable:**
Complete, integrated e-commerce platform

---

## Development Environment Setup

### Prerequisites
- Go 1.21 or higher
- Docker & Docker Compose
- Protocol Buffers compiler (`protoc`)
- `protoc-gen-go` and `protoc-gen-go-grpc` plugins

### Local Development Stack (Docker Compose)

```yaml
version: '3.8'

services:
  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    environment:
      MONGO_INITDB_DATABASE: ecommerce

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: ecommerce
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres

  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
    depends_on:
      - zookeeper
```

### Service Ports
- API Gateway: `:8080` (HTTP)
- Product Service: `:50051` (gRPC)
- Cart Service: `:50052` (gRPC)
- Inventory Service: `:50053` (gRPC)
- Payment Service: `:50054` (gRPC) - stub implemented
- Orders Service: `:50055` (gRPC) - planned
- Checkout Service: `:50056` (gRPC) - planned

---

## Getting Started

### Step 1: Initialize Workspace
```bash
mkdir ecommerce-platform && cd ecommerce-platform
go work init
```

### Step 2: Start Infrastructure
```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### Step 3: Build Cart Service (First Service)
```bash
mkdir -p cart-service/{cmd/server,internal/{domain,repository,cache,service,grpc,consumer},pkg/proto}
cd cart-service
go mod init github.com/yourusername/ecommerce-platform/cart-service
```

### Step 4: Install Dependencies
```bash
go get go.mongodb.org/mongo-driver/mongo
go get github.com/redis/go-redis/v9
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get modernc.org/sqlite
```

### Step 5: Define Protobuf Schema
Create `pkg/proto/cart.proto` with service definitions

### Step 6: Generate Code
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       pkg/proto/cart.proto
```

### Step 7: Implement Service
- Domain entities
- Repository layer (MongoDB)
- Cache layer (Redis)
- Business logic service
- gRPC handlers
- Main server

### Step 8: Repeat for Other Services
Follow the same pattern for Product, Checkout, Orders, Inventory, and Payment services

### Step 9: Build API Gateway
Connect all services through HTTP REST endpoints

### Step 10: Integration Testing
Use provided test scripts to verify end-to-end functionality

---

## Configuration Management

### Environment Variables (Per Service)

**Cart Service:**
```bash
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=ecommerce
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
GRPC_PORT=8081
KAFKA_BROKERS=localhost:9092
```

**Checkout Service:**
```bash
POSTGRES_URI=postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable
GRPC_PORT=8082
INVENTORY_SERVICE_ADDR=localhost:8085
PAYMENT_SERVICE_ADDR=localhost:8086
KAFKA_BROKERS=localhost:9092
```

**Orders Service:**
```bash
POSTGRES_URI=postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable
GRPC_PORT=8083
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=orders-service
```

**API Gateway:**
```bash
HTTP_PORT=8080
CART_SERVICE_ADDR=localhost:8081
CHECKOUT_SERVICE_ADDR=localhost:8082
ORDERS_SERVICE_ADDR=localhost:8083
PRODUCT_SERVICE_ADDR=localhost:8084
JWT_SECRET=your-secret-key
```

---

## Data Flow Examples

### Example 1: Add Item to Cart
```
1. User sends POST /api/v1/cart/items with:
   - product_id: 1
   - quantity: 2

2. API Gateway:
   - Validates JWT token → extracts user_id
   - Calls CartService.AddItem(user_id, product_id, quantity) via gRPC

3. Cart Service:
   - Calls ProductService.ValidateProducts([1]) via gRPC
   - If valid: Updates cart in MongoDB
   - Invalidates Redis cache for user_id
   - Returns success

4. API Gateway returns HTTP 201 Created
```

### Example 2: View Cart
```
1. User sends GET /api/v1/cart

2. API Gateway:
   - Validates JWT → extracts user_id
   - Calls CartService.GetCart(user_id) via gRPC

3. Cart Service:
   - Checks Redis cache for "cart:{user_id}"
   - If cache miss:
     * Queries MongoDB for cart
     * Stores in Redis with 15min TTL
   - Returns cart with product_ids and quantities

4. API Gateway:
   - Calls ProductService.GetProducts([product_ids]) via gRPC
   - Merges cart items with product details (name, price, image)
   - Calculates subtotals and total
   - Returns enriched cart to user
```

### Example 3: Checkout Flow
```
1. User sends POST /api/v1/checkout with:
   - shipping_address
   - payment_method
   - idempotency_key: "uuid-from-client"

2. API Gateway:
   - Validates JWT → extracts user_id
   - Calls CheckoutService.InitiateCheckout() via gRPC

3. Checkout Service (Saga):
   
   Step 1: Create Session
   - Check if session with idempotency_key exists
   - If exists: return existing result
   - If not: create new session in PostgreSQL (status: INITIATED)
   - Fetch user's cart from Cart Service
   - Store cart snapshot in checkout session
   
   Step 2: Reserve Inventory
   - Call InventoryService.Reserve(checkout_id, cart_items, ttl=5min)
   - If fails: update status to FAILED, return error
   - If success: store reservation_id, update status to INVENTORY_RESERVED
   
   Step 3: Process Payment
   - Call PaymentService.Charge(checkout_id, total_amount, payment_method)
   - If fails:
     * Call InventoryService.Release(reservation_id)
     * Update status to FAILED
     * Return error to user
   - If success: store payment_id, update status to PAYMENT_COMPLETED
   
   Step 4: Publish Event
   - Insert into outbox_events table (transactional with status update):
     {
       aggregate_id: checkout_id,
       event_type: "CheckoutCompleted",
       payload: { checkout_id, user_id, items, total_amount }
     }
   - Update status to COMPLETED
   
   Step 5: Return Success
   - Return checkout_id and status to user

4. Background: Outbox Poller
   - Polls outbox_events every 1 second
   - Finds unprocessed events (processed_at IS NULL)
   - Publishes to Kafka topic "checkout-events"
   - Marks as processed

5. Orders Service (Async):
   - Consumes "CheckoutCompleted" from Kafka
   - Checks if order with checkout_id exists (idempotency)
   - Creates order record in PostgreSQL
   - Commits Kafka offset

6. Cart Service (Async):
   - Consumes "CheckoutCompleted" from Kafka
   - Deletes cart for user_id from MongoDB
   - Invalidates Redis cache
   - Commits Kafka offset
```

---

## Testing Strategy

### Unit Tests
Each service should have unit tests for:
- Domain logic
- Repository layer (use testcontainers or mocks)
- Service layer
- gRPC handlers

### Integration Tests
- Test service-to-service communication
- Test database operations with real databases (testcontainers)
- Test Kafka producer/consumer with embedded Kafka

### End-to-End Tests
Full workflow testing:
1. Add items to cart
2. View cart with enriched data
3. Complete checkout
4. Verify order created
5. Verify cart cleared

### Load Tests
- Cart service read/write throughput
- Checkout service under concurrent requests
- Redis cache hit rates
- Kafka throughput

---

## Observability

### Logging
- Structured logging (JSON format)
- Log levels: DEBUG, INFO, WARN, ERROR
- Context propagation (request_id, user_id, trace_id)

### Metrics
- Request latency (p50, p95, p99)
- Error rates
- Cache hit/miss rates
- Database query performance
- Kafka lag (consumer groups)
- Service availability (uptime)

### Tracing
- Distributed tracing with OpenTelemetry
- Trace complete request flow across services
- Visualize in Jaeger or similar

### Health Checks
Each service exposes:
- `/health/live` - Liveness probe
- `/health/ready` - Readiness probe (checks dependencies)

---

## Scaling Considerations

### Horizontal Scaling
All services are stateless and can be scaled horizontally:
- Cart Service: Multiple instances behind load balancer
- Checkout Service: Multiple instances (saga state in PostgreSQL)
- Orders Service: Multiple Kafka consumer instances (same group)
- API Gateway: Multiple instances behind load balancer

### Database Scaling
- **MongoDB**: Replica sets for read scaling, sharding for write scaling
- **PostgreSQL**: Read replicas, partitioning
- **Redis**: Redis Cluster or Redis Sentinel

### Kafka Scaling
- Multiple partitions per topic
- Consumer group for parallel processing
- Partition key = user_id for ordering guarantees

### Caching Strategy
- Redis for hot data (frequently accessed carts)
- CDN for product images
- API Gateway response caching (for product catalog)

---

## Security Considerations

### Authentication & Authorization
- JWT tokens for user authentication
- Token validation in API Gateway
- User context propagation to backend services via gRPC metadata

### Data Security
- TLS/SSL for all external communications
- mTLS for inter-service gRPC (production)
- Encrypted connections to databases
- Sensitive data encryption at rest (payment info)

### API Security
- Rate limiting per user/IP
- Request size limits
- Input validation and sanitization
- CORS configuration

### Secrets Management
- Environment variables for local development
- Kubernetes Secrets or HashiCorp Vault for production
- Never commit credentials to version control

---

## Future Enhancements

### Phase 5: Advanced Features
- Order tracking and notifications
- Email/SMS notifications
- Inventory management UI
- Admin dashboard
- Analytics and reporting

### Phase 6: Performance Optimization
- Database query optimization
- Connection pooling tuning
- Cache warming strategies
- Circuit breakers and fallbacks

### Phase 7: Resilience
- Chaos engineering tests
- Disaster recovery procedures
- Multi-region deployment
- Database backups and restore procedures

### Phase 8: Observability Enhancement
- Custom dashboards (Grafana)
- Alerting rules (Prometheus Alertmanager)
- Log aggregation (ELK stack)
- APM integration (Datadog, New Relic)

---

## Conclusion

This high-level implementation plan provides a comprehensive blueprint for building an enterprise-grade e-commerce platform in Go using microservices architecture. The system emphasizes:

- **Clear separation of concerns** with dedicated services
- **Event-driven architecture** for loose coupling
- **Saga pattern** for distributed transaction management
- **Caching strategies** for performance
- **Observability** for production readiness

By following this plan incrementally (phases 1-4), you'll build a functional system while learning:
- Go microservices patterns
- gRPC communication
- Event-driven architecture with Kafka
- Database design (MongoDB, PostgreSQL)
- Caching strategies (Redis)
- Distributed systems concepts

The stub services (Product, Inventory, Payment) allow focusing on the core architecture while providing realistic integration points that can be replaced with real implementations later.