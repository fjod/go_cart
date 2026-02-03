# Integration Test Flow Plan

## Overview
This document describes the complete integration test flow for the e-commerce platform, covering all API Gateway endpoints and verifying the full checkout saga orchestration.

---

## Test Environment Setup

### Prerequisites
1. **Infrastructure Services Running:**
   ```bash
   docker-compose -f deployments/docker-compose.dev.yml up -d
   ```
   Verify:
   - MongoDB: localhost:27017 (Cart data)
   - Redis: localhost:6379 (Cart cache)
   - PostgreSQL: localhost:5432 (Checkout, Inventory, Payment)

2. **Microservices Running (in separate terminals):**
   ```bash
   # Terminal 1 - Product Service
   go run ./product-service/cmd/main.go     # :50051

   # Terminal 2 - Cart Service
   go run ./cart-service/cmd/main.go        # :50052

   # Terminal 3 - Inventory Service
   go run ./inventory-service/cmd/main.go   # :50053

   # Terminal 4 - Payment Service
   go run ./payment-service/cmd/main.go     # :50054

   # Terminal 5 - Checkout Service
   go run ./checkout-service/main.go        # :50056

   # Terminal 6 - API Gateway
   go run ./api-gateway/cmd/main.go         # :8080
   ```

3. **Test Data:**
   - Product Service auto-seeds 5 products (Laptop, Mouse, Keyboard, Monitor, Headphones)
   - Inventory Service initializes with stock for all products
   - User ID: 1 (provided by MockAuthMiddleware)

4. **PostgreSQL MCP Server (For Agent Automation):**
   - MCP server connection is available for automated database verification
   - Sub-agents can use MCP tools to verify checkout_sessions and outbox_events tables
   - All verification queries are documented in each test phase below
   - Database: PostgreSQL on localhost:5432, schema: public

---

## Agent Automation Guidelines

When using the integration-flow-validator agent or similar sub-agents, follow this verification pattern:

1. **Execute API request** (curl command from test phase)
2. **Validate HTTP response** (status code, JSON structure, expected values)
3. **Verify database state** (use MCP tools to query PostgreSQL)
4. **Check event propagation** (verify outbox_events table for async operations)

**Example Verification Flow:**
```
Step 1: POST /api/v1/checkout
  → Capture checkout_id from response

Step 2: Verify checkout_sessions table
  → mcp__postgres-mcp__execute_sql(
      sql: "SELECT status FROM checkout_sessions WHERE id = '<checkout_id>'"
    )
  → Assert: status = 'COMPLETED' or 'FAILED'

Step 3: Verify outbox_events table
  → mcp__postgres-mcp__execute_sql(
      sql: "SELECT event_type FROM outbox_events WHERE aggregate_id = '<checkout_id>'"
    )
  → Assert: event_type matches expected event
```

---

## Test Flow Sequence

### Phase 1: Health & Discovery

#### Test 1.1: Health Check
**Purpose:** Verify API Gateway is running

```bash
curl -X GET http://localhost:8080/health
```

**Expected Response:**
```json
{
  "status": "ok"
}
```

**Status Code:** `200 OK`

---

### Phase 2: Product Catalog

#### Test 2.1: List All Products
**Purpose:** Verify Product Service integration and retrieve product catalog

```bash
curl -X GET http://localhost:8080/api/v1/products
```

**Expected Response:**
```json
{
  "products": [
    {
      "id": 1,
      "name": "Laptop",
      "description": "High-performance laptop",
      "price": 1299.99,
      "image_url": "http://example.com/laptop.jpg",
      "created_at": "2026-02-01T..."
    },
    {
      "id": 2,
      "name": "Mouse",
      "description": "Wireless mouse",
      "price": 29.99,
      "image_url": "http://example.com/mouse.jpg",
      "created_at": "2026-02-01T..."
    }
    // ... 3 more products (Keyboard, Monitor, Headphones)
  ]
}
```

**Status Code:** `200 OK`

**Validation:**
- Response contains exactly 5 products
- All products have id, name, price
- Prices are positive numbers

---

### Phase 3: Cart Operations

#### Test 3.1: Get Empty Cart
**Purpose:** Verify initial cart state

```bash
curl -X GET http://localhost:8080/api/v1/cart
```

**Expected Response:**
```json
{
  "cart_id": "",
  "user_id": 1,
  "items": [],
  "total_items": 0,
  "total_price": 0,
  "updated_at": "0001-01-01T00:00:00Z"
}
```

**Status Code:** `200 OK`

---

#### Test 3.2: Add First Item to Cart
**Purpose:** Test cart creation and item addition

```bash
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "quantity": 2
  }'
```

**Expected Response:**
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    }
  ],
  "total_items": 1,
  "total_price": 2599.98,
  "updated_at": "2026-02-03T..."
}
```

**Status Code:** `200 OK`

**Validation:**
- cart_id is non-empty MongoDB ObjectId
- items array contains 1 item
- subtotal = price * quantity (1299.99 * 2)
- total_price matches subtotal

---

#### Test 3.3: Add Second Item to Cart
**Purpose:** Test adding different product to existing cart

```bash
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 2,
    "quantity": 1
  }'
```

**Expected Response:**
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    },
    {
      "product_id": 2,
      "product_name": "Mouse",
      "quantity": 1,
      "price": 29.99,
      "subtotal": 29.99
    }
  ],
  "total_items": 2,
  "total_price": 2629.97,
  "updated_at": "2026-02-03T..."
}
```

**Status Code:** `200 OK`

**Validation:**
- Same cart_id as Test 3.2
- items array contains 2 items
- total_price = 2599.98 + 29.99 = 2629.97

---

#### Test 3.4: Update Item Quantity
**Purpose:** Test quantity modification

```bash
curl -X PUT http://localhost:8080/api/v1/cart/items/2 \
  -H "Content-Type: application/json" \
  -d '{
    "quantity": 3
  }'
```

**Expected Response:**
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    },
    {
      "product_id": 2,
      "product_name": "Mouse",
      "quantity": 3,
      "price": 29.99,
      "subtotal": 89.97
    }
  ],
  "total_items": 2,
  "total_price": 2689.95,
  "updated_at": "2026-02-03T..."
}
```

**Status Code:** `200 OK`

**Validation:**
- Mouse quantity updated from 1 → 3
- Mouse subtotal = 29.99 * 3 = 89.97
- total_price = 2599.98 + 89.97 = 2689.95

---

#### Test 3.5: Get Cart (Verify Cache)
**Purpose:** Test cache retrieval after modifications

```bash
curl -X GET http://localhost:8080/api/v1/cart
```

**Expected Response:**
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    },
    {
      "product_id": 2,
      "product_name": "Mouse",
      "quantity": 3,
      "price": 29.99,
      "subtotal": 89.97
    }
  ],
  "total_items": 2,
  "total_price": 2689.95,
  "updated_at": "2026-02-03T..."
}
```

**Status Code:** `200 OK`

**Validation:**
- Matches response from Test 3.4
- Verifies cache invalidation worked

---

#### Test 3.6: Remove Item from Cart
**Purpose:** Test item deletion

```bash
curl -X DELETE http://localhost:8080/api/v1/cart/items/2
```

**Expected Response:**
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    }
  ],
  "total_items": 1,
  "total_price": 2599.98,
  "updated_at": "2026-02-03T..."
}
```

**Status Code:** `200 OK`

**Validation:**
- Mouse (product_id: 2) removed
- Only Laptop remains
- total_price reverted to Laptop subtotal

---

### Phase 4: Error Handling

#### Test 4.1: Add Invalid Product
**Purpose:** Verify product validation

```bash
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 999,
    "quantity": 1
  }'
```

**Expected Response:**
```json
{
  "error": "product not found"
}
```

**Status Code:** `404 Not Found` or `400 Bad Request`

---

#### Test 4.2: Add Item with Invalid Quantity
**Purpose:** Verify quantity validation

```bash
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "quantity": 0
  }'
```

**Expected Response:**
```json
{
  "error": "quantity must be between 1 and 99"
}
```

**Status Code:** `400 Bad Request`

---

#### Test 4.3: Update Non-existent Item
**Purpose:** Verify item existence validation

```bash
curl -X PUT http://localhost:8080/api/v1/cart/items/999 \
  -H "Content-Type: application/json" \
  -d '{
    "quantity": 5
  }'
```

**Expected Response:**
```json
{
  "error": "item not found in cart"
}
```

**Status Code:** `404 Not Found`

---

### Phase 5: Checkout Flow (Happy Path)

#### Test 5.1: Successful Checkout
**Purpose:** Test complete checkout saga orchestration

**Pre-condition:** Cart contains 2x Laptop (from Test 3.6)

```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "test-checkout-001"
  }'
```

**Expected Response:**
```json
{
  "checkout_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED"
}
```

**Status Code:** `200 OK`

**Validation:**
- checkout_id is valid UUID
- status is "COMPLETED" (payment succeeded)

**Backend Verification:**
1. Inventory Service: 2 laptops reserved and confirmed
2. Payment Service: Payment processed successfully
3. Checkout Session: Status = COMPLETED in PostgreSQL
4. Outbox Events: Event written to outbox_events table

**MCP Database Verification (For Agent):**
```
# Verify checkout session exists and has correct status
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, user_id, status, total_amount, idempotency_key
        FROM checkout_sessions
        WHERE id = '<checkout_id>'"
)

# Expected result:
# - status = 'COMPLETED'
# - user_id = 1
# - total_amount = 2599.98 (2x Laptop)
# - idempotency_key = 'test-checkout-001'

# Verify outbox event was created
mcp__postgres-mcp__execute_sql(
  sql: "SELECT event_type, aggregate_id, processed_at
        FROM outbox_events
        WHERE aggregate_id = '<checkout_id>'
        ORDER BY created_at DESC LIMIT 1"
)

# Expected result:
# - event_type = 'checkout.completed'
# - aggregate_id = checkout_id from response
# - processed_at should be NOT NULL (event published)
```

---

#### Test 5.2: Verify Cart Cleared After Checkout
**Purpose:** Verify cart cleanup (eventual consistency)

```bash
# Wait 2 seconds for async event processing
sleep 2

curl -X GET http://localhost:8080/api/v1/cart
```

**Expected Response:**
```json
{
  "cart_id": "",
  "user_id": 1,
  "items": [],
  "total_items": 0,
  "total_price": 0,
  "updated_at": "0001-01-01T00:00:00Z"
}
```

**Status Code:** `200 OK`

**Validation:**
- Cart is empty after successful checkout
- Verifies Kafka event consumed by Cart Service

---

### Phase 6: Checkout Idempotency

#### Test 6.1: Idempotent Retry
**Purpose:** Verify idempotency prevents duplicate processing

```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "test-checkout-001"
  }'
```

**Expected Response:**
```json
{
  "checkout_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED"
}
```

**Status Code:** `200 OK`

**Validation:**
- Returns SAME checkout_id as Test 5.1
- No duplicate payment processing
- No duplicate inventory deduction

---

### Phase 7: Checkout Error Scenarios

#### Test 7.1: Checkout with Empty Cart
**Purpose:** Verify empty cart validation

**Pre-condition:** Cart is empty (from Test 5.2)

```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "test-checkout-empty-001"
  }'
```

**Expected Response:**
```json
{
  "error": "cart is empty"
}
```

**Status Code:** `400 Bad Request` or `500 Internal Server Error`

---

#### Test 7.2: Checkout with Missing Idempotency Key
**Purpose:** Verify idempotency key requirement

```bash
# Re-add item to cart first
curl -X POST http://localhost:8080/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 3,
    "quantity": 1
  }'

# Try checkout without idempotency_key
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Expected Response:**
```json
{
  "error": "idempotency_key is required"
}
```

**Status Code:** `400 Bad Request`

---

#### Test 7.3: Checkout with Payment Failure (Simulated)
**Purpose:** Verify saga compensation on payment failure

**Note:** Payment Service has 95% success rate. Run multiple times to trigger failure, or temporarily configure Payment Service for 100% failure rate.

```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{
    "idempotency_key": "test-checkout-fail-001"
  }'
```

**Expected Response (on payment failure):**
```json
{
  "checkout_id": "650e8400-e29b-41d4-a716-446655440001",
  "status": "FAILED"
}
```

**Status Code:** `200 OK`

**Backend Verification:**
1. Inventory Service: Reservation released (compensated)
2. Checkout Session: Status = FAILED in PostgreSQL
3. Cart: NOT cleared (remains intact)

**MCP Database Verification (For Agent):**
```
# Verify checkout session failed correctly
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, status, total_amount, failure_reason
        FROM checkout_sessions
        WHERE id = '<checkout_id>'"
)

# Expected result:
# - status = 'FAILED'
# - failure_reason should contain payment error details

# Verify compensation event exists (optional, if implemented)
mcp__postgres-mcp__execute_sql(
  sql: "SELECT event_type, aggregate_id
        FROM outbox_events
        WHERE aggregate_id = '<checkout_id>'
        AND event_type LIKE '%failed%'
        ORDER BY created_at DESC LIMIT 1"
)
```

---

### Phase 8: Cart Cleanup

#### Test 8.1: Clear Cart
**Purpose:** Test cart reset functionality

```bash
curl -X DELETE http://localhost:8080/api/v1/cart
```

**Expected Response:**
```json
{
  "cart_id": "",
  "user_id": 1,
  "items": [],
  "total_items": 0,
  "total_price": 0,
  "updated_at": "0001-01-01T00:00:00Z"
}
```

**Status Code:** `200 OK`

**Validation:**
- Cart completely cleared
- Cache invalidated


**PostgreSQL MCP Server (For Agent Automation):**

The PostgreSQL MCP server is available for automated verification during integration testing. Use these MCP tools to verify database state:

1. **List schemas:**
   ```
   mcp__postgres-mcp__list_schemas
   ```

2. **List tables in public schema:**
   ```
   mcp__postgres-mcp__list_objects(schema_name: "public", object_type: "table")
   ```

3. **Get table details:**
   ```
   mcp__postgres-mcp__get_object_details(schema_name: "public", object_name: "checkout_sessions", object_type: "table")
   ```

4. **Execute SQL queries:**
   ```
   mcp__postgres-mcp__execute_sql(sql: "SELECT id, user_id, status, total_amount FROM checkout_sessions ORDER BY created_at DESC LIMIT 1")
   ```

5. **Verify checkout session by ID:**
   ```
   mcp__postgres-mcp__execute_sql(sql: "SELECT * FROM checkout_sessions WHERE id = '<checkout_id>'")
   ```

6. **Verify outbox events:**
   ```
   mcp__postgres-mcp__execute_sql(sql: "SELECT event_type, aggregate_id, processed_at FROM outbox_events ORDER BY created_at DESC LIMIT 5")
   ```

### Service Logs

Monitor each service terminal for:
- gRPC method calls
- Error messages
- Saga state transitions
- Cache hits/misses
- Kafka event publishing/consuming

---

## Test Metrics & Success Criteria

### Functional Requirements
- ✅ All 14 test cases pass
- ✅ Cart operations complete in < 200ms (cache hit)
- ✅ Checkout completes in < 5s (including payment processing)
- ✅ Idempotency prevents duplicate processing
- ✅ Saga compensation releases inventory on payment failure
- ✅ Cart cleared after successful checkout (eventual consistency)

### Non-Functional Requirements
- ✅ API Gateway handles 1000+ req/s for reads
- ✅ Redis cache hit rate > 80% for cart operations
- ✅ Zero data loss during normal operation
- ✅ Graceful degradation on service failures

---

## Troubleshooting

### Common Issues

**1. "connection refused" errors:**
- Verify all services are running on correct ports
- Check firewall rules
- Verify gRPC service addresses in environment variables

**2. "cart is empty" during checkout:**
- Cart was cleared by previous test
- Re-add items before checkout

**3. Cart not cleared after checkout:**
- Kafka broker not running
- Cart Service not consuming events
- Check `docker-compose` logs

**4. Payment always fails:**
- Inventory Service out of stock
- Payment Service configured for 100% failure rate
- Check Payment Service logs

**5. Stale cart data:**
- Redis cache not invalidated
- Known issue: async cache invalidation race condition
- Wait 50-100ms after mutations

---

## Appendix: Request/Response Examples

### Add Item Request Body
```json
{
  "product_id": 1,
  "quantity": 2
}
```

### Update Quantity Request Body
```json
{
  "quantity": 5
}
```

### Checkout Request Body
```json
{
  "idempotency_key": "unique-key-20260203-001"
}
```

### Full Cart Response
```json
{
  "cart_id": "507f1f77bcf86cd799439011",
  "user_id": 1,
  "items": [
    {
      "product_id": 1,
      "product_name": "Laptop",
      "quantity": 2,
      "price": 1299.99,
      "subtotal": 2599.98
    }
  ],
  "total_items": 1,
  "total_price": 2599.98,
  "updated_at": "2026-02-03T10:30:00Z"
}
```

### Checkout Response
```json
{
  "checkout_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED"
}
```

---

## Appendix: PostgreSQL MCP Verification Queries

### Common Verification Patterns for Agents

**1. Verify Checkout Session Status:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, user_id, status, total_amount, idempotency_key, created_at
        FROM checkout_sessions
        WHERE id = '<checkout_id>'"
)
```

**2. Get Latest Checkout Session:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, status, total_amount
        FROM checkout_sessions
        ORDER BY created_at DESC
        LIMIT 1"
)
```

**3. Verify Outbox Event Processing:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT event_type, aggregate_id, processed_at, created_at
        FROM outbox_events
        WHERE aggregate_id = '<checkout_id>'
        ORDER BY created_at DESC"
)
```

**4. Count Checkout Sessions by Status:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT status, COUNT(*) as count
        FROM checkout_sessions
        GROUP BY status"
)
```

**5. Verify Idempotency Key:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, status
        FROM checkout_sessions
        WHERE idempotency_key = '<idempotency_key>'"
)
```

**6. Check for Failed Checkouts:**
```
mcp__postgres-mcp__execute_sql(
  sql: "SELECT id, failure_reason, created_at
        FROM checkout_sessions
        WHERE status = 'FAILED'
        ORDER BY created_at DESC
        LIMIT 5"
)
```

**7. Verify Table Structure:**
```
mcp__postgres-mcp__get_object_details(
  schema_name: "public",
  object_name: "checkout_sessions",
  object_type: "table"
)
```

**8. List All Tables:**
```
mcp__postgres-mcp__list_objects(
  schema_name: "public",
  object_type: "table"
)
```

---

**Document Version:** 1.1
**Last Updated:** February 3, 2026
**Maintained By:** Development Team
**Changelog:**
- v1.1: Added PostgreSQL MCP server integration for automated testing
- v1.0: Initial integration test flow documentation
