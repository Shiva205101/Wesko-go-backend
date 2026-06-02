package observability

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Configure enables W3C trace-context propagation and initializes a basic
// tracer provider so that Trace IDs are generated for all requests.
func Configure(service string) {
	// Set up resource (metadata about the service)
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
		),
	)

	// Set up TracerProvider (Generates the actual IDs)
	// For now, we use AlwaysSample so every request gets an ID.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Set up Propagators (allows IDs to travel across services)
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
