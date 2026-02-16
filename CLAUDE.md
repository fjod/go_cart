# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A distributed e-commerce platform in Go with microservices architecture for learning purposes.

**Current Phase:** Phase 4 (Integration & Polish) - Not Started
**Completed:** Phase 1 (Foundation) ✅ | Phase 2 (Checkout Orchestration) ✅ | Phase 3 (Order Processing) ✅

## Services Running

| Service | Port | Tech Stack | Status |
|---------|------|------------|--------|
| Product Service | :50051 (gRPC) | SQLite | ✅ Complete |
| Cart Service | :50052 (gRPC) | MongoDB + Redis + Kafka | ✅ Complete |
| Inventory Service | :50053 (gRPC) | In-memory | ✅ Complete |
| Payment Service | :50054 (gRPC) | Mock | ✅ Complete |
| Orders Service | :50055 (gRPC) | PostgreSQL + Kafka | ✅ Complete |
| Checkout Service | :50056 (gRPC) | PostgreSQL + Kafka | ✅ Complete |
| API Gateway | :8080 (HTTP) | chi router | ✅ Complete (10 routes) |

## Architecture

```
                   ┌─────────────────────────┐
                   │   API Gateway :8080     │
                   │      (chi router)       │
                   └───┬──────────────────┬──┘
                       │                  │
       ┌───────────────┼──────────────────┼──────────────┐
       │               │                  │              │
  ┌────▼────┐    ┌─────▼────┐      ┌─────▼────┐  ┌─────▼────┐
  │ Product │    │   Cart   │      │ Checkout │  │  Orders  │
  │ :50051  │◄───│  :50052  │      │  :50056  │  │  :50055  │
  │ SQLite  │    │  Mongo   │      │ Postgres │  │ Postgres │
  └─────────┘    │  Redis   │      └────┬─────┘  └────▲─────┘
                 └────▲─────┘           │              │
                      │            ┌────▼─────┐        │
                      │            │ Payment  │        │
                      │            │  :50054  │        │
                      │            └──────────┘        │
                      │                                │
                      └────────── Kafka ───────────────┘
                              (checkout-outbox)
```

## Build & Run

### Start Infrastructure
```bash
# MongoDB, Redis, PostgreSQL, Kafka (KRaft mode), Kafdrop UI
docker-compose -f deployments/docker-compose.dev.yml up -d
```

### Run Services
```bash
# From workspace root
go run ./product-service/cmd/main.go    # gRPC :50051
go run ./cart-service/cmd/main.go       # gRPC :50052
go run ./inventory-service/cmd/main.go  # gRPC :50053
go run ./payment-service/cmd/main.go    # gRPC :50054
go run ./orders-service/cmd/main.go     # gRPC :50055
go run ./checkout-service/main.go       # gRPC :50056
go run ./api-gateway/cmd/main.go        # HTTP :8080
```

### Run Tests
```powershell
.\test-all.ps1              # All services
.\test-all.ps1 -Verbose     # With output
.\test-all.ps1 -Short       # Skip integration tests
.\test-all.ps1 -Cover       # With coverage

# Individual service
go test -v ./cart-service/...
go test -v -run TestFunctionName ./cart-service/internal/grpc/...
```

### Generate Proto Files
```bash
.\product-service\generate.bat
.\cart-service\genProto.bat
.\checkout-service\genProto.bat
.\inventory-service\genProto.bat
.\payment-service\genProto.bat
.\orders-service\genProto.bat
```

## API Endpoints (Gateway)

```
GET    /health                        Health check
GET    /api/v1/products               List products
GET    /api/v1/cart                   Get user cart
POST   /api/v1/cart/items             Add item
PUT    /api/v1/cart/items/{id}        Update quantity
DELETE /api/v1/cart/items/{id}        Remove item
DELETE /api/v1/cart                   Clear cart (idempotent)
POST   /api/v1/checkout               Initiate checkout (saga orchestration)
GET    /api/v1/orders                 List user's orders
GET    /api/v1/orders/{order_id}      Get order by ID
```

## Key Architectural Patterns

### Cache-Aside with Singleflight (Cart Service)
- Redis → MongoDB on miss → populate cache async
- `golang.org/x/sync/singleflight` prevents cache stampede
- TTL: 15 min base + 0-5 min jitter
- Write-through invalidation on mutations

### Saga Orchestration (Checkout Service)
1. Create Session (idempotency check)
2. Reserve Inventory (sync gRPC call)
3. Process Payment (sync gRPC call)
4. Complete Checkout (transactional outbox pattern)
5. Publish Event to Kafka → Cart Service clears cart + Orders Service creates order (async)

**Compensation:** Payment failure → Release inventory → Mark FAILED

### Transactional Outbox Pattern
- Atomic write: checkout_sessions status + outbox_events insert
- Background poller (1s tick) publishes events to Kafka topic `checkout-outbox`
- Recovery mechanism for stuck sessions (5s tick)

### Event-Driven Cart Clearing & Order Creation
- Cart Service: Kafka consumer (`cart-service-consumer` group) clears cart on `CheckoutCompleted`
- Orders Service: Kafka consumer (`orders-service` group) creates order on `CheckoutCompleted`
- Both consumers use `StartOffset: kafka.FirstOffset` to avoid cold-start message loss

### Testing
- Unit tests with mocks
- Integration tests with testcontainers (MongoDB, Redis, Kafka, PostgreSQL)
- End-to-end tests: 19/25 passing (v1.4)

## Environment Variables

| Variable | Service | Default |
|----------|---------|---------|
| HTTP_PORT | Gateway | 8080 |
| GRPC_PORT | Product/Cart/Inventory/Payment | 50051-50054 |
| CHECKOUT_SERVICE_PORT | Checkout | 50056 |
| MONGO_URI | Cart | mongodb://localhost:27017 |
| REDIS_ADDR | Cart | localhost:6379 |
| KAFKA_ADDR | Cart | localhost:9092 |
| KAFKA_BROKERS | Checkout, Orders | localhost:9092 |
| DB_HOST, DB_PORT, DB_NAME | Checkout, Orders | localhost, 5432, ecommerce |
| PRODUCT_SERVICE_ADDR | Cart, Gateway, Checkout | localhost:50051 |
| CART_SERVICE_ADDR | Gateway, Checkout | localhost:50052 |
| INVENTORY_SERVICE_ADDR | Checkout | localhost:50053 |
| PAYMENT_SERVICE_ADDR | Checkout | localhost:50054 |
| CHECKOUT_SERVICE_ADDR | Gateway | localhost:50056 |
| ORDERS_SERVICE_ADDR | Gateway | localhost:50055 |

## Service Structure Pattern

```
service/
├── cmd/main.go              # Entry point
├── domain/                  # Entities, DTOs, state machines
├── internal/
│   ├── repository/          # Data access + migrations
│   ├── cache/               # Redis (Cart only)
│   ├── service/             # Business logic
│   ├── grpc/                # gRPC handlers
│   ├── publisher/           # Outbox poller (Checkout only)
│   ├── consumer/            # Kafka consumer (Orders only)
│   ├── poller/              # Kafka consumer (Cart only)
│   └── store/               # In-memory (Inventory only)
└── pkg/proto/               # Protobuf definitions
```

## Current Status

### Phase 1: Foundation ✅ Complete
- Product Service: 2 endpoints (GetProducts, GetProduct)
- Cart Service: 5 endpoints (Add, Get, Update, Remove, Clear) with Redis caching + Kafka consumer
- API Gateway: 10 REST endpoints

### Phase 2: Checkout Orchestration ✅ Complete
- Checkout Service: Full saga orchestration (4 steps) with compensation
- Inventory Service: In-memory stock management with reservations
- Payment Service: Mock payment processing (95% success rate)
- Outbox poller: event publishing + stuck session recovery
- Kafka infrastructure: KRaft broker + Kafdrop UI

### Phase 3: Order Processing ✅ Complete
- Orders Service: Kafka consumer + PostgreSQL persistence + gRPC query API (GetOrder, ListOrders)
- Idempotent event processing via `checkout_id` UNIQUE constraint

### Phase 4: Integration & Polish ❌ Not Started
- Distributed tracing (OpenTelemetry)
- Real JWT authentication (replace MockAuthMiddleware)
- Rate limiting middleware
- Circuit breakers
- Structured logging
- End-to-end observability

## Known Issues

1. **Async cache invalidation race**: Cache may serve stale data immediately after mutations (workaround: 50ms sleep in tests)
2. **JWT authentication**: MockAuthMiddleware in Gateway always injects user_id=1
3. **Integration test flakiness**: Tests 5.2/5.3 can fail if Kafka consumer group join latency exceeds test wait window (fixed in code with `FirstOffset`; restart services if observed)

## Next Priorities

1. Replace MockAuthMiddleware with real JWT validation
2. Add distributed tracing (OpenTelemetry)
3. Add rate limiting middleware to API Gateway
4. Implement circuit breakers for backend service calls
5. Add structured logging (replace fmt.Printf with slog/zap)

## Testing

- **Total tests:** 49+ unit and integration tests across all services
- **Integration tests:** 19/25 passing (v1.4 flow; 4 failures: Kafka timing + cart schema deviations)
- **Test coverage:** Repository, Service, gRPC handler, Kafka consumer layers

See `integration_test_flow.md` for full test suite documentation.

## References

- `HIGH_LEVEL_IMPLEMENTATION_PLAN.md` - Complete architecture blueprint
- `project_status.md` - Detailed implementation tracking
- `integration_test_flow.md` - End-to-end test cases (v1.4, 25 assertions)
