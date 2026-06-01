package observability

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Configure enables W3C trace-context propagation so upstream trace headers
// can flow through the service and into downstream calls.
func Configure() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
}

func HTTPMiddleware(service string) gin.HandlerFunc {
	propagators := otel.GetTextMapPropagator()
	tracer := otel.Tracer(service)

	return func(c *gin.Context) {
		ctx := propagators.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		spanName := c.Request.Method + " " + c.Request.URL.Path
		ctx, span := tracer.Start(ctx, spanName, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if route := c.FullPath(); route != "" {
			span.SetName(c.Request.Method + " " + route)
			span.SetAttributes(attribute.String("http.route", route))
		}

		status := c.Writer.Status()
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("url.path", c.Request.URL.Path),
			attribute.Int("http.status_code", status),
		)
		if len(c.Errors) > 0 || status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, c.Errors.String())
		}
		span.End()
	}
}
