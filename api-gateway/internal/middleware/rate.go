package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	clients sync.Map
	rate    rate.Limit
	burst   int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:  rate.Limit(rps),
		burst: burst,
	}
	// Cleanup stale entries periodically
	go rl.cleanup(10 * time.Minute)
	return rl
}

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	if v, ok := rl.clients.Load(key); ok {
		c := v.(*client)
		c.lastSeen = time.Now()
		return c.limiter
	}
	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.clients.Store(key, &client{limiter: limiter, lastSeen: time.Now()})
	return limiter
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	for {
		time.Sleep(interval)
		rl.clients.Range(func(key, value any) bool {
			c := value.(*client)
			if time.Since(c.lastSeen) > 3*time.Minute {
				rl.clients.Delete(key)
			}
			return true
		})
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use user ID from JWT if available, fall back to IP
		key := r.RemoteAddr // or extract from your auth context
		if userID := getUserIDFromContext(r.Context()); userID != "" {
			key = userID
		}

		if !rl.getLimiter(key).Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getUserIDFromContext(context context.Context) string {
	contextValue := context.Value(UserIDKey)
	if userID, ok := contextValue.(string); ok {
		return userID
	}
	return ""
}
