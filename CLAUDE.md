# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A distributed e-commerce platform in Go with microservices architecture for learning purposes. Currently in Phase 1 with three services: Product Service, Cart Service, and API Gateway.

## Build & Run Commands

### Running Services
```bash
# From workspace root - run each service
go run ./product-service/cmd/main.go    # gRPC :50051
go run ./cart-service/cmd/main.go       # gRPC :50052
go run ./api-gateway/cmd/main.go        # HTTP :8080
```

### Infrastructure (required for Cart Service)
```bash
docker-compose -f deployments/docker-compose.dev.yml up -d  # MongoDB + Redis
```

### Running Tests
```powershell
# All modules (PowerShell)
.\test-all.ps1                    # Standard run
.\test-all.ps1 -Verbose           # With verbose output
.\test-all.ps1 -Short             # Skip integration tests (uses -short flag)
.\test-all.ps1 -Cover             # With coverage
.\test-all.ps1 -Timeout 600       # 10 min timeout (default 5 min)

# Individual module
go test -v ./cart-service/...
go test -v ./product-service/...
go test -v ./api-gateway/...

# Single test
go test -v -run TestFunctionName ./cart-service/internal/grpc/...
```

### Generating Proto Files
```bash
# Windows batch scripts in each service directory
.\product-service\generate.bat
.\cart-service\genProto.bat
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway (HTTP :8080)                  │
│                         chi router                           │
└───────────┬──────────────────────┬──────────────────────────┘
            │ gRPC                 │ gRPC
    ┌───────▼────────┐     ┌───────▼────────┐
    │  Cart Service  │────▶│ Product Service│
    │   gRPC :50052  │     │  gRPC :50051   │
    │ MongoDB+Redis  │     │    SQLite      │
    └────────────────┘     └────────────────┘
```

### Service Structure Pattern
Each service follows:
```
service/
├── cmd/main.go              # Entry point, dependency wiring
├── internal/
│   ├── domain/              # Entities (Cart, Product)
│   ├── repository/          # Data access (MongoDB, SQLite)
│   ├── cache/               # Redis cache layer (Cart only)
│   ├── service/             # Business logic with caching (Cart only)
│   └── grpc/                # gRPC handlers
│       ├── handler.go
│       ├── handler_test.go
│       └── handler_integration_test.go
└── pkg/proto/               # Protobuf definitions and generated code
```

### Key Patterns

**Cache-Aside with Singleflight** (Cart Service):
- Check Redis → DB on miss → populate cache async
- `golang.org/x/sync/singleflight` prevents cache stampede
- TTL: 15 min base + 0-5 min jitter
- Write-through invalidation on mutations

**Service-to-Service**: Cart validates products exist via gRPC call to Product Service

**Testing**: Uses testcontainers-go for real MongoDB/Redis in integration tests

## Environment Variables

| Variable | Service | Default |
|----------|---------|---------|
| GRPC_PORT | Product | :50051 |
| GRPC_PORT | Cart | :50052 |
| HTTP_PORT | Gateway | 8080 |
| MONGO_URI | Cart | mongodb://localhost:27017 |
| MONGO_DB_NAME | Cart | cartdb |
| REDIS_ADDR | Cart | localhost:6379 |
| PRODUCT_SERVICE_ADDR | Cart, Gateway | localhost:50051 |
| CART_SERVICE_ADDR | Gateway | localhost:50052 |
| DB_PATH | Product | internal/repository/products.db |
| MIGRATIONS_PATH | Product | internal/repository/migrations |

## API Endpoints (Gateway)

```
GET    /health                    Health check
GET    /api/v1/products          List products
GET    /api/v1/cart              Get user cart
POST   /api/v1/cart/items        Add item (body: product_id, quantity)
PUT    /api/v1/cart/items/{id}   Update quantity
DELETE /api/v1/cart/items/{id}   Remove item
DELETE /api/v1/cart              Clear cart
```

## Known Issues

- **Async cache invalidation race**: Cache may serve stale data immediately after mutations. Tests use 50ms sleep as workaround. Fix: sync invalidation or read-your-writes pattern.

## Future Services (Phases 2+)

Checkout, Orders, Inventory, and Payment services planned per HIGH_LEVEL_IMPLEMENTATION_PLAN.md. Will add Kafka for event-driven communication.
Also check current status in project_status.md file.
