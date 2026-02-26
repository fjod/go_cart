package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
)

type KafkaHeaderCarrier struct {
	headers map[string]string
}

func (p KafkaHeaderCarrier) Get(key string) string {
	return p.headers[key]
}

func (p KafkaHeaderCarrier) Set(key, value string) {
	p.headers[key] = value
}

func (p KafkaHeaderCarrier) Keys() []string {
	ret := make([]string, 0, len(p.headers))
	for k := range p.headers {
		ret = append(ret, k)
	}
	return ret
}

// Inject injects the tracing from the context into a string map
func Inject(ctx context.Context) map[string]string {
	h := &KafkaHeaderCarrier{
		headers: make(map[string]string),
	}
	otel.GetTextMapPropagator().Inject(ctx, h)
	return h.headers
}

// Extract extracts the tracing from a string map into the context
func Extract(ctx context.Context, headers map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, &KafkaHeaderCarrier{headers: headers})
}
