package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Breaker wraps a gobreaker.CircuitBreaker and provides a gRPC unary client interceptor.
type Breaker struct {
	cb *gobreaker.CircuitBreaker[any]
}

// Settings configures the circuit breaker behavior.
type Settings struct {
	// Name identifies the circuit breaker in logs and metrics.
	Name string

	// MaxRequests is the number of requests allowed in half-open state.
	// If 0, only 1 request is allowed.
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for clearing internal counts.
	// If 0, counts are never cleared while closed.
	Interval time.Duration

	// Timeout is how long the breaker stays open before transitioning to half-open.
	Timeout time.Duration

	// FailureThreshold is the number of consecutive failures before opening.
	FailureThreshold uint32

	// Logger is optional. If nil, slog.Default() is used.
	Logger *slog.Logger
}

// DefaultSettings returns sensible defaults for a gRPC circuit breaker.
// Open after 5 consecutive failures, stay open for 10 seconds, allow 3 probes in half-open.
func DefaultSettings(name string, logger *slog.Logger) Settings {
	return Settings{
		Name:             name,
		MaxRequests:      3,
		Interval:         60 * time.Second,
		Timeout:          10 * time.Second,
		FailureThreshold: 5,
		Logger:           logger,
	}
}

// New creates a Breaker with the given settings.
func New(s Settings) *Breaker {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}

	threshold := s.FailureThreshold
	if threshold == 0 {
		threshold = 5
	}

	gbSettings := gobreaker.Settings{
		Name:        s.Name,
		MaxRequests: s.MaxRequests,
		Interval:    s.Interval,
		Timeout:     s.Timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= threshold
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("circuit breaker state change",
				slog.String("breaker", name),
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
		},

		// Only count real infrastructure failures, not business errors.
		// gRPC codes like NotFound, InvalidArgument, AlreadyExists are NOT failures —
		// they mean the server responded normally.
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			st, ok := status.FromError(err)
			if !ok {
				// Not a gRPC error — treat as failure (network error, etc.)
				return false
			}
			switch st.Code() {
			case codes.OK,
				codes.Canceled,         // Client canceled — not a server problem
				codes.InvalidArgument,  // Bad request — server is fine
				codes.NotFound,         // Resource missing — server is fine
				codes.AlreadyExists,    // Duplicate — server is fine
				codes.PermissionDenied, // Auth issue — server is fine
				codes.Unauthenticated,  // Auth issue — server is fine
				codes.FailedPrecondition:
				return true
			default:
				// Unavailable, DeadlineExceeded, Internal, ResourceExhausted, etc.
				// These indicate the server is struggling.
				return false
			}
		},
	}

	return &Breaker{
		cb: gobreaker.NewCircuitBreaker[any](gbSettings),
	}
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that wraps
// every outgoing call with the circuit breaker.
//
// When the circuit is open, calls fail immediately with codes.Unavailable
// without ever hitting the network.
func (b *Breaker) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		_, err := b.cb.Execute(func() (any, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			return nil, err
		})

		if err != nil {
			// Translate gobreaker errors to gRPC status codes so callers
			// can handle them uniformly.
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return status.Errorf(codes.Unavailable,
					"circuit breaker %s is open: %v", b.cb.Name(), err)
			}
			return err
		}
		return nil
	}
}

// State returns the current state of the circuit breaker (closed, half-open, open).
func (b *Breaker) State() gobreaker.State {
	return b.cb.State()
}

// Counts returns the current internal counts (requests, successes, failures).
func (b *Breaker) Counts() gobreaker.Counts {
	return b.cb.Counts()
}

// String returns a human-readable representation of the breaker state.
func (b *Breaker) String() string {
	counts := b.cb.Counts()
	return fmt.Sprintf("breaker[%s] state=%s requests=%d failures=%d consecutive_failures=%d",
		b.cb.Name(), b.cb.State(), counts.Requests, counts.TotalFailures, counts.ConsecutiveFailures)
}
