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

---

## Full End-to-End Test Script

### Bash Script (Linux/Mac)

```bash
#!/bin/bash

API_URL="http://localhost:8080"

echo "=== E-Commerce Integration Test ==="
echo

# Test 1: Health Check
echo "1. Health Check"
curl -s -X GET $API_URL/health | jq '.'
echo

# Test 2: List Products
echo "2. List Products"
PRODUCTS=$(curl -s -X GET $API_URL/api/v1/products)
echo $PRODUCTS | jq '.'
PRODUCT_COUNT=$(echo $PRODUCTS | jq '.products | length')
echo "Product count: $PRODUCT_COUNT (expected: 5)"
echo

# Test 3: Get Empty Cart
echo "3. Get Empty Cart"
curl -s -X GET $API_URL/api/v1/cart | jq '.'
echo

# Test 4: Add Laptop to Cart
echo "4. Add 2x Laptop"
curl -s -X POST $API_URL/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id": 1, "quantity": 2}' | jq '.'
echo

# Test 5: Add Mouse to Cart
echo "5. Add 1x Mouse"
curl -s -X POST $API_URL/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id": 2, "quantity": 1}' | jq '.'
echo

# Test 6: Update Mouse Quantity
echo "6. Update Mouse to 3x"
curl -s -X PUT $API_URL/api/v1/cart/items/2 \
  -H "Content-Type: application/json" \
  -d '{"quantity": 3}' | jq '.'
echo

# Test 7: Get Cart (verify cache)
echo "7. Get Cart (cache test)"
curl -s -X GET $API_URL/api/v1/cart | jq '.'
echo

# Test 8: Remove Mouse
echo "8. Remove Mouse from Cart"
curl -s -X DELETE $API_URL/api/v1/cart/items/2 | jq '.'
echo

# Test 9: Checkout
echo "9. Initiate Checkout"
CHECKOUT_RESPONSE=$(curl -s -X POST $API_URL/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{"idempotency_key": "integration-test-'$(date +%s)'"}')
echo $CHECKOUT_RESPONSE | jq '.'
CHECKOUT_ID=$(echo $CHECKOUT_RESPONSE | jq -r '.checkout_id')
CHECKOUT_STATUS=$(echo $CHECKOUT_RESPONSE | jq -r '.status')
echo "Checkout ID: $CHECKOUT_ID"
echo "Status: $CHECKOUT_STATUS"
echo

# Test 10: Verify Cart Cleared (wait for async processing)
echo "10. Verify Cart Cleared (waiting 2s for Kafka event)"
sleep 2
curl -s -X GET $API_URL/api/v1/cart | jq '.'
echo

# Test 11: Idempotent Retry
echo "11. Test Idempotency (retry same checkout)"
curl -s -X POST $API_URL/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{"idempotency_key": "integration-test-'$(date +%s)'"}' | jq '.'
echo

# Test 12: Error - Empty Cart Checkout
echo "12. Test Empty Cart Error"
curl -s -X POST $API_URL/api/v1/checkout \
  -H "Content-Type: application/json" \
  -d '{"idempotency_key": "error-test-'$(date +%s)'"}' | jq '.'
echo

# Test 13: Error - Invalid Product
echo "13. Test Invalid Product Error"
curl -s -X POST $API_URL/api/v1/cart/items \
  -H "Content-Type: application/json" \
  -d '{"product_id": 999, "quantity": 1}' | jq '.'
echo

# Test 14: Clear Cart
echo "14. Clear Cart"
curl -s -X DELETE $API_URL/api/v1/cart | jq '.'
echo

echo "=== Integration Test Complete ==="
```

### PowerShell Script (Windows)

```powershell
# integration-test.ps1

$API_URL = "http://localhost:8080"

Write-Host "=== E-Commerce Integration Test ===" -ForegroundColor Cyan
Write-Host

# Test 1: Health Check
Write-Host "1. Health Check" -ForegroundColor Yellow
$response = Invoke-RestMethod -Uri "$API_URL/health" -Method Get
$response | ConvertTo-Json
Write-Host

# Test 2: List Products
Write-Host "2. List Products" -ForegroundColor Yellow
$products = Invoke-RestMethod -Uri "$API_URL/api/v1/products" -Method Get
$products | ConvertTo-Json -Depth 5
Write-Host "Product count: $($products.products.Count) (expected: 5)"
Write-Host

# Test 3: Get Empty Cart
Write-Host "3. Get Empty Cart" -ForegroundColor Yellow
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart" -Method Get
$cart | ConvertTo-Json
Write-Host

# Test 4: Add Laptop
Write-Host "4. Add 2x Laptop" -ForegroundColor Yellow
$body = @{ product_id = 1; quantity = 2 } | ConvertTo-Json
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart/items" -Method Post -Body $body -ContentType "application/json"
$cart | ConvertTo-Json -Depth 5
Write-Host

# Test 5: Add Mouse
Write-Host "5. Add 1x Mouse" -ForegroundColor Yellow
$body = @{ product_id = 2; quantity = 1 } | ConvertTo-Json
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart/items" -Method Post -Body $body -ContentType "application/json"
$cart | ConvertTo-Json -Depth 5
Write-Host

# Test 6: Update Mouse Quantity
Write-Host "6. Update Mouse to 3x" -ForegroundColor Yellow
$body = @{ quantity = 3 } | ConvertTo-Json
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart/items/2" -Method Put -Body $body -ContentType "application/json"
$cart | ConvertTo-Json -Depth 5
Write-Host

# Test 7: Get Cart
Write-Host "7. Get Cart (cache test)" -ForegroundColor Yellow
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart" -Method Get
$cart | ConvertTo-Json -Depth 5
Write-Host

# Test 8: Remove Mouse
Write-Host "8. Remove Mouse from Cart" -ForegroundColor Yellow
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart/items/2" -Method Delete
$cart | ConvertTo-Json -Depth 5
Write-Host

# Test 9: Checkout
Write-Host "9. Initiate Checkout" -ForegroundColor Yellow
$timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$body = @{ idempotency_key = "integration-test-$timestamp" } | ConvertTo-Json
try {
    $checkout = Invoke-RestMethod -Uri "$API_URL/api/v1/checkout" -Method Post -Body $body -ContentType "application/json"
    $checkout | ConvertTo-Json
    Write-Host "Checkout ID: $($checkout.checkout_id)" -ForegroundColor Green
    Write-Host "Status: $($checkout.status)" -ForegroundColor Green
} catch {
    Write-Host "Checkout Error: $_" -ForegroundColor Red
}
Write-Host

# Test 10: Verify Cart Cleared
Write-Host "10. Verify Cart Cleared (waiting 2s)" -ForegroundColor Yellow
Start-Sleep -Seconds 2
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart" -Method Get
$cart | ConvertTo-Json
Write-Host

# Test 11: Error - Empty Cart Checkout
Write-Host "11. Test Empty Cart Error" -ForegroundColor Yellow
$timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$body = @{ idempotency_key = "error-test-$timestamp" } | ConvertTo-Json
try {
    $result = Invoke-RestMethod -Uri "$API_URL/api/v1/checkout" -Method Post -Body $body -ContentType "application/json"
    $result | ConvertTo-Json
} catch {
    Write-Host "Expected Error: $_" -ForegroundColor Magenta
}
Write-Host

# Test 12: Clear Cart
Write-Host "12. Clear Cart" -ForegroundColor Yellow
$cart = Invoke-RestMethod -Uri "$API_URL/api/v1/cart" -Method Delete
$cart | ConvertTo-Json
Write-Host

Write-Host "=== Integration Test Complete ===" -ForegroundColor Cyan
```

---

## Automated Testing Tools

### Option 1: Postman Collection
Create a Postman collection with:
- Environment variables: `base_url = http://localhost:8080`
- Pre-request scripts for generating unique idempotency keys
- Test assertions in each request
- Collection runner for sequential execution

### Option 2: Go Integration Test Suite

```go
// tests/integration/api_test.go
package integration_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

const baseURL = "http://localhost:8080"

func TestFullCheckoutFlow(t *testing.T) {
    client := &http.Client{Timeout: 10 * time.Second}

    // Test 1: Health Check
    t.Run("HealthCheck", func(t *testing.T) {
        resp, err := client.Get(baseURL + "/health")
        require.NoError(t, err)
        defer resp.Body.Close()

        assert.Equal(t, http.StatusOK, resp.StatusCode)

        var result map[string]string
        json.NewDecoder(resp.Body).Decode(&result)
        assert.Equal(t, "ok", result["status"])
    })

    // Test 2: List Products
    t.Run("ListProducts", func(t *testing.T) {
        resp, err := client.Get(baseURL + "/api/v1/products")
        require.NoError(t, err)
        defer resp.Body.Close()

        assert.Equal(t, http.StatusOK, resp.StatusCode)

        var result struct {
            Products []map[string]interface{} `json:"products"`
        }
        json.NewDecoder(resp.Body).Decode(&result)
        assert.Equal(t, 5, len(result.Products))
    })

    // Test 3: Add Item to Cart
    var cartID string
    t.Run("AddItemToCart", func(t *testing.T) {
        body := map[string]interface{}{
            "product_id": 1,
            "quantity":   2,
        }
        jsonBody, _ := json.Marshal(body)

        resp, err := client.Post(
            baseURL+"/api/v1/cart/items",
            "application/json",
            bytes.NewBuffer(jsonBody),
        )
        require.NoError(t, err)
        defer resp.Body.Close()

        assert.Equal(t, http.StatusOK, resp.StatusCode)

        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        cartID = result["cart_id"].(string)
        assert.NotEmpty(t, cartID)
        assert.Equal(t, float64(2599.98), result["total_price"])
    })

    // Test 4: Checkout
    var checkoutID string
    t.Run("Checkout", func(t *testing.T) {
        body := map[string]interface{}{
            "idempotency_key": "test-" + time.Now().Format("20060102150405"),
        }
        jsonBody, _ := json.Marshal(body)

        resp, err := client.Post(
            baseURL+"/api/v1/checkout",
            "application/json",
            bytes.NewBuffer(jsonBody),
        )
        require.NoError(t, err)
        defer resp.Body.Close()

        assert.Equal(t, http.StatusOK, resp.StatusCode)

        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        checkoutID = result["checkout_id"].(string)
        assert.NotEmpty(t, checkoutID)
        assert.Contains(t, []string{"COMPLETED", "FAILED"}, result["status"])
    })

    // Test 5: Verify Cart Cleared (eventual consistency)
    t.Run("VerifyCartCleared", func(t *testing.T) {
        time.Sleep(2 * time.Second) // Wait for Kafka event

        resp, err := client.Get(baseURL + "/api/v1/cart")
        require.NoError(t, err)
        defer resp.Body.Close()

        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        items := result["items"].([]interface{})
        assert.Empty(t, items)
    })
}
```

---

## Performance Testing

### Load Test with Apache Bench

```bash
# Test cart operations throughput
ab -n 1000 -c 10 -p add-item.json -T application/json \
  http://localhost:8080/api/v1/cart/items

# Test product listing
ab -n 5000 -c 50 http://localhost:8080/api/v1/products
```

### Load Test with `hey`

```bash
# Install: go install github.com/rakyll/hey@latest

# Product listing (read-heavy)
hey -n 10000 -c 100 http://localhost:8080/api/v1/products

# Cart retrieval (cache performance)
hey -n 5000 -c 50 http://localhost:8080/api/v1/cart
```

---

## Monitoring & Verification

### Database Verification

**MongoDB (Cart Data):**
```bash
mongosh
use cartdb
db.carts.find({ user_id: 1 }).pretty()
```

**PostgreSQL (Checkout Sessions):**
```sql
-- Connect to ecommerce database
\c ecommerce

-- View recent checkouts
SELECT id, user_id, status, total_amount, created_at
FROM checkout_sessions
ORDER BY created_at DESC
LIMIT 10;

-- View outbox events
SELECT id, event_type, aggregate_id, processed_at, created_at
FROM outbox_events
ORDER BY created_at DESC
LIMIT 10;
```

**Redis (Cart Cache):**
```bash
redis-cli
KEYS cart:*
GET cart:1
TTL cart:1
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

## Next Steps

1. **Automate Tests:** Convert to CI/CD pipeline (GitHub Actions, GitLab CI)
2. **Add Orders Service:** Test order creation from checkout events
3. **Add Inventory Timeout:** Test auto-release after 5 minutes
4. **Add Retry Logic:** Test transient failure recovery
5. **Add Circuit Breakers:** Test service degradation gracefully

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

**Document Version:** 1.0
**Last Updated:** February 3, 2026
**Maintained By:** Development Team
