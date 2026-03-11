package circuitbreaker

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNew_DefaultSettings(t *testing.T) {
	s := DefaultSettings("test-service", slog.Default())
	b := New(s)

	if b.State() != gobreaker.StateClosed {
		t.Errorf("expected closed state, got %s", b.State())
	}

	counts := b.Counts()
	if counts.Requests != 0 {
		t.Errorf("expected 0 requests, got %d", counts.Requests)
	}
}

func TestIsSuccessful_BusinessErrors(t *testing.T) {
	// Business errors (server responded fine) should NOT trip the breaker.
	s := DefaultSettings("test-service", slog.Default())
	s.FailureThreshold = 2
	s.Timeout = 50 * time.Millisecond
	b := New(s)

	businessErrors := []codes.Code{
		codes.NotFound,
		codes.InvalidArgument,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
	}

	interceptor := b.UnaryClientInterceptor()

	for _, code := range businessErrors {
		grpcErr := status.Errorf(code, "test error")
		// Create a fake invoker that returns this error
		invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return grpcErr
		}
		err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
		if err == nil {
			t.Errorf("expected error for code %s, got nil", code)
		}
	}

	// After all those "business" errors, breaker should still be closed.
	if b.State() != gobreaker.StateClosed {
		t.Errorf("expected breaker to stay closed after business errors, got %s", b.State())
	}
}

func TestIsSuccessful_InfraErrors_TripBreaker(t *testing.T) {
	s := DefaultSettings("test-service", slog.Default())
	s.FailureThreshold = 3
	s.Timeout = 100 * time.Millisecond
	b := New(s)

	interceptor := b.UnaryClientInterceptor()

	// Simulate infrastructure failures (Unavailable = server is down)
	invokerDown := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Errorf(codes.Unavailable, "connection refused")
	}

	// Send enough failures to trip the breaker
	for i := 0; i < 3; i++ {
		_ = interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invokerDown)
	}

	if b.State() != gobreaker.StateOpen {
		t.Errorf("expected open state after 3 infra failures, got %s", b.State())
	}

	// Next call should be rejected immediately without hitting the invoker
	invokerShouldNotBeCalled := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		t.Fatal("invoker should not be called when breaker is open")
		return nil
	}

	err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invokerShouldNotBeCalled)
	if err == nil {
		t.Fatal("expected error when breaker is open, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable code, got %s", st.Code())
	}
}

func TestBreaker_RecoveryAfterTimeout(t *testing.T) {
	s := DefaultSettings("test-service", slog.Default())
	s.FailureThreshold = 2
	s.Timeout = 100 * time.Millisecond // Short timeout for testing
	s.MaxRequests = 1
	b := New(s)

	interceptor := b.UnaryClientInterceptor()

	// Trip the breaker
	invokerDown := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Errorf(codes.Unavailable, "down")
	}
	for i := 0; i < 2; i++ {
		_ = interceptor(context.Background(), "/test/Method", nil, nil, nil, invokerDown)
	}

	if b.State() != gobreaker.StateOpen {
		t.Fatalf("expected open, got %s", b.State())
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	if b.State() != gobreaker.StateHalfOpen {
		t.Fatalf("expected half-open after timeout, got %s", b.State())
	}

	// Successful probe should close the breaker
	invokerOK := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}
	err := interceptor(context.Background(), "/test/Method", nil, nil, nil, invokerOK)
	if err != nil {
		t.Fatalf("expected nil error on recovery probe, got %v", err)
	}

	if b.State() != gobreaker.StateClosed {
		t.Errorf("expected closed after successful probe, got %s", b.State())
	}
}

func TestBreaker_DeadlineExceededTripsBreaker(t *testing.T) {
	s := DefaultSettings("test-service", slog.Default())
	s.FailureThreshold = 2
	s.Timeout = 100 * time.Millisecond
	b := New(s)

	interceptor := b.UnaryClientInterceptor()

	invokerSlow := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return status.Errorf(codes.DeadlineExceeded, "timeout")
	}

	for i := 0; i < 2; i++ {
		_ = interceptor(context.Background(), "/test/Method", nil, nil, nil, invokerSlow)
	}

	if b.State() != gobreaker.StateOpen {
		t.Errorf("expected open after deadline exceeded errors, got %s", b.State())
	}
}

func TestBreaker_String(t *testing.T) {
	s := DefaultSettings("test-service", slog.Default())
	b := New(s)

	str := b.String()
	if str == "" {
		t.Error("expected non-empty string representation")
	}

	// Should contain the name and state
	if !contains(str, "test-service") || !contains(str, "closed") {
		t.Errorf("unexpected string output: %s", str)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
