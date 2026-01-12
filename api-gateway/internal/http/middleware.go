package http

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// MockAuthMiddleware simulates JWT authentication (replace with real JWT validation)
func MockAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production: validate JWT token from Authorization header
		// Extract user_id from token claims

		// For demo, use hardcoded user_id for testing
		// In production, this would come from parsing and validating the JWT token
		var userID int64 = 1

		// Add user_id to context as int64 (matches handler expectations)
		ctx := context.WithValue(r.Context(), "user_id", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}

		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
