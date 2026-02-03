# Checkout Service Integration Guide

## Overview
The checkout service is now fully integrated with gRPC and the API gateway. It orchestrates a distributed transaction across Cart, Product, Inventory, and Payment services using the saga pattern.

## Architecture

```
HTTP Request → API Gateway → Checkout Service (gRPC)
                                    ↓
                    ┌───────────────┴───────────────┐
                    ↓               ↓               ↓               ↓
               Cart Service   Product Service   Inventory      Payment
               (validate)     (get prices)      (reserve)      (process)
```

## Starting the Services

### 1. Start Infrastructure
```bash
docker-compose -f deployments/docker-compose.dev.yml up -d
# Starts: MongoDB (Cart), Redis (Cart Cache), PostgreSQL (Checkout, Inventory, Payment)
```

### 2. Start All Services (in separate terminals)

```bash
# Terminal 1 - Product Service (port 50051)
go run ./product-service/cmd/main.go

# Terminal 2 - Cart Service (port 50052)
go run ./cart-service/cmd/main.go

# Terminal 3 - Inventory Service (port 50053)
go run ./inventory-service/cmd/main.go

# Terminal 4 - Payment Service (port 50054)
go run ./payment-service/cmd/main.go

# Terminal 5 - Checkout Service (port 50056)
go run ./checkout-service/main.go
# Should see: "Checkout service listening on :50056"

# Terminal 6 - API Gateway (port 8080)
go run ./api-gateway/cmd/main.go
```

## Environment Variables

### Checkout Service
```bash
GRPC_PORT=50056                              # Default: 50056
CART_SERVICE_ADDR=localhost:50052            # Cart service
PRODUCT_SERVICE_ADDR=localhost:50051         # Product service
INVENTORY_SERVICE_ADDR=localhost:50053       # Inventory service
PAYMENT_SERVICE_ADDR=localhost:50054         # Payment service
DB_HOST=localhost                            # PostgreSQL host
DB_PORT=5432                                 # PostgreSQL port
DB_USER=postgres                             # Database user
DB_PASSWORD=postgres                         # Database password
DB_NAME=ecommerce                            # Database name
MIGRATIONS_PATH=./internal/repository/migrations
```

### API Gateway
```bash
HTTP_PORT=8080                               # Default: 8080
CHECKOUT_SERVICE_ADDR=localhost:50056        # NEW: Checkout service
CART_SERVICE_ADDR=localhost:50052
PRODUCT_SERVICE_ADDR=localhost:50051
```

## Testing End-to-End Checkout Flow

### Step 1: Add Items to Cart
```bash
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "quantity": 2
  }'

# Response: {"cart_id":"...","items":[...],"total_price":...}
```

### Step 2: Initiate Checkout
```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "unique-checkout-20260201-001"
  }'

# Successful Response:
{
  "checkout_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED"
}

# Failed Response (e.g., payment declined):
{
  "checkout_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "FAILED"
}
```

### Step 3: Test Idempotency
```bash
# Repeat the same request with identical idempotency_key
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "unique-checkout-20260201-001"
  }'

# Should return the SAME checkout_id and status as Step 2
```

## Status Values

The checkout process progresses through these statuses:

1. **INITIATED** - Checkout session created
2. **INVENTORY_RESERVED** - Items reserved in inventory
3. **PAYMENT_PENDING** - Payment being processed
4. **PAYMENT_COMPLETED** - Payment successful
5. **COMPLETED** - Checkout fully complete, cart cleared
6. **FAILED** - Checkout failed (inventory released if needed)

## Saga Compensation

The checkout service implements automatic compensation on failures:

- **Payment fails after inventory reserved** → Inventory automatically released
- **Empty cart** → Returns error, no side effects
- **Product not found** → Returns error, no side effects
- **Idempotent retry** → Returns original result, no duplicate processing

## Direct gRPC Testing (Optional)

If you have `grpcurl` installed:

```bash
# List available services
grpcurl -plaintext localhost:50056 list
# Output: checkout.CheckoutService

# Describe the service
grpcurl -plaintext localhost:50056 describe checkout.CheckoutService

# Call InitiateCheckout
grpcurl -plaintext -d '{
  "user_id": 1,
  "idempotency_key": "test-grpc-001"
}' localhost:50056 checkout.CheckoutService/InitiateCheckout
```

## Database Schema

The checkout service uses PostgreSQL with the following tables:

- **checkout_sessions** - Main checkout state
- **checkout_inventory_items** - Reserved inventory items per checkout
- **checkout_payments** - Payment records per checkout
- **outbox_messages** - Event outbox for eventual consistency

Check `checkout-service/internal/repository/migrations/` for schema definitions.

## Error Handling

### Empty Cart
```bash
# Try checkout with empty cart
curl -X DELETE http://localhost:8080/api/v1/cart  # Clear cart first
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{"idempotency_key": "empty-test-001"}'

# Returns: 500 Internal Server Error
# Error: "cart is empty"
```

### Missing Idempotency Key
```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{}'

# Returns: 400 Bad Request
# Error: "idempotency_key is required"
```

## Key Implementation Details

### gRPC Handler (`checkout-service/internal/grpc/handler.go`)
- Validates input (user_id > 0, idempotency_key required)
- Bridges gRPC proto types to domain types
- Handles nil pointer safety for domain response fields

### Business Logic (`checkout-service/internal/service/checkout_service.go`)
- Orchestrates 4-service saga pattern
- Implements idempotency via database lookup
- Compensates on payment failure by releasing inventory

### API Gateway Handler (`api-gateway/internal/http/checkout_handler.go`)
- Extracts user_id from context (mock auth provides user_id=1)
- Validates JSON request body
- Maps proto status enums to human-readable strings

## Next Steps

Future enhancements could include:

1. **Async processing** - Return checkout_id immediately, process in background
2. **Webhooks** - Notify external systems on completion
3. **Order creation** - Create order record on successful checkout
4. **Inventory timeout** - Auto-release reservations after X minutes
5. **Retry logic** - Automatic retry on transient failures
