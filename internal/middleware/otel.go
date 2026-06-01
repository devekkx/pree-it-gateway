package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter           = otel.Meter("chat.gateway")
	httpReqCounter  metric.Int64Counter
	httpReqDuration metric.Float64Histogram
)

func init() {
	var err error
	httpReqCounter, err = meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		panic(err)
	}

	httpReqDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in ms"),
		metric.WithUnit("ms"),
		// Explicit buckets matching Grafana's default latency panels
		metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000),
	)
	if err != nil {
		panic(err)
	}
}

// Tracing wraps otelgin for automatic span creation per request.
func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}

// Metrics records RED (Rate, Errors, Duration) metrics per route.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []attribute.KeyValue{
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", c.FullPath()),
			attribute.Int("http.status_code", c.Writer.Status()),
		}

		httpReqCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
		httpReqDuration.Record(c.Request.Context(),
			float64(time.Since(start).Milliseconds()),
			metric.WithAttributes(attrs...),
		)
	}
}
