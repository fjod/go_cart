# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A distributed e-commerce platform in Go with microservices architecture for learning purposes.

**Current Phase:** Phase 2 (Checkout Orchestration) - 97% complete
**Next Phase:** Phase 3 (Order Processing with Kafka)

## Services Running

| Service | Port | Tech Stack | Status |
|---------|------|------------|--------|
| Product Service | :50051 (gRPC) | SQLite | ✅ Complete |
| Cart Service | :50052 (gRPC) | MongoDB + Redis | ✅ Complete |
| Inventory Service | :50053 (gRPC) | In-memory | ✅ Complete |
| Payment Service | :50054 (gRPC) | Mock | ✅ Complete |
| Checkout Service | :50056 (gRPC) | PostgreSQL | 🔄 97% (Kafka pending) |
| API Gateway | :8080 (HTTP) | chi router | ✅ Complete |

## Architecture

```
                   ┌─────────────────────────┐
                   │   API Gateway :8080     │
                   │      (chi router)       │
                   └───┬─────────────────┬───┘
                       │                 │
         ┌─────────────┼─────────────────┼─────────────┐
         │             │                 │             │
    ┌────▼────┐   ┌───▼────┐      ┌─────▼────┐  ┌────▼─────┐
    │ Product │   │  Cart  │      │Checkout  │  │Inventory │
    │ :50051  │◄──│ :50052 │      │ :50056   │  │  :50053  │
    │ SQLite  │   │ Mongo  │      │ Postgres │  │ In-mem   │
    └─────────┘   │ Redis  │      └────┬─────┘  └──────────┘
                  └────────┘           │
                                  ┌────▼─────┐
                                  │ Payment  │
                                  │  :50054  │
                                  │   Mock   │
                                  └──────────┘
```

## Build & Run

### Start Infrastructure
```bash
# MongoDB, Redis, PostgreSQL
docker-compose -f deployments/docker-compose.dev.yml up -d
```

### Run Services
```bash
# From workspace root
go run ./product-service/cmd/main.go    # gRPC :50051
go run ./cart-service/cmd/main.go       # gRPC :50052
go run ./inventory-service/cmd/main.go  # gRPC :50053
go run ./payment-service/cmd/main.go    # gRPC :50054
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
```

## API Endpoints (Gateway)

```
GET    /health                    Health check
GET    /api/v1/products          List products
GET    /api/v1/cart              Get user cart
POST   /api/v1/cart/items        Add item
PUT    /api/v1/cart/items/{id}   Update quantity
DELETE /api/v1/cart/items/{id}   Remove item
DELETE /api/v1/cart              Clear cart
POST   /api/v1/checkout          Initiate checkout (saga orchestration)
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
5. Publish Event to Kafka → Cart Service clears cart (async)

**Compensation:** Payment failure → Release inventory → Mark FAILED

### Transactional Outbox Pattern
- Atomic write: checkout_sessions status + outbox_events insert
- Background poller publishes events to Kafka
- Recovery mechanism for stuck sessions (5s interval)

### Testing
- Unit tests with mocks
- Integration tests with testcontainers (MongoDB, Redis)
- End-to-end tests: 14/16 passing

## Environment Variables

| Variable | Service | Default |
|----------|---------|---------|
| HTTP_PORT | Gateway | 8080 |
| GRPC_PORT | Product/Cart/Inventory/Payment | 50051-50054 |
| CHECKOUT_SERVICE_PORT | Checkout | 50056 |
| MONGO_URI | Cart | mongodb://localhost:27017 |
| REDIS_ADDR | Cart | localhost:6379 |
| DB_HOST, DB_PORT, DB_NAME | Checkout | localhost, 5432, ecommerce |
| PRODUCT_SERVICE_ADDR | Cart, Gateway, Checkout | localhost:50051 |
| CART_SERVICE_ADDR | Gateway, Checkout | localhost:50052 |
| INVENTORY_SERVICE_ADDR | Checkout | localhost:50053 |
| PAYMENT_SERVICE_ADDR | Checkout | localhost:50054 |
| CHECKOUT_SERVICE_ADDR | Gateway | localhost:50056 |

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
│   └── store/               # In-memory (Inventory only)
└── pkg/proto/               # Protobuf definitions
```

## Current Status

### Phase 1: Foundation ✅ Complete
- Product Service: 2 endpoints (GetProducts, GetProduct)
- Cart Service: 5 endpoints (Add, Get, Update, Remove, Clear) with Redis caching
- API Gateway: 7 REST endpoints

### Phase 2: Checkout Orchestration 🔄 97% Complete
- Checkout Service: Full saga orchestration with compensation
- Inventory Service: Stock management with reservations
- Payment Service: Mock payment processing (95% success rate)
- **Completed:** Saga steps 1-4, outbox recovery mechanism (7 tests passing)
- **Pending:** Kafka event publishing (processUnpublishedEvents implementation)

### Phase 3: Order Processing ❌ Not Started
- Orders Service (Kafka consumer)
- Order status tracking

## Known Issues

1. **Async cache invalidation race**: Cache may serve stale data immediately after mutations (workaround: 50ms sleep in tests)
2. **Kafka infrastructure**: Not launched yet - cart clearing after checkout pending
3. **JWT authentication**: MockAuthMiddleware in Gateway (always user_id=1)

## Next Priorities

1. Implement Kafka event publishing in outbox poller
2. Set up Kafka infrastructure (docker-compose)
3. Add Kafka consumer to Cart Service for clearing carts
4. Build Orders Service for Phase 3
5. Replace MockAuthMiddleware with real JWT validation

## Testing

- **Total tests:** 30+ unit tests across all services
- **Integration tests:** 14/16 passing (2 expected failures: Kafka not running)
- **Test coverage:** Repository, Service, gRPC handler layers

See `integration_test_flow.md` for full test suite documentation.

## References

- `HIGH_LEVEL_IMPLEMENTATION_PLAN.md` - Complete architecture blueprint
- `project_status.md` - Detailed implementation tracking
- `integration_test_flow.md` - End-to-end test cases
