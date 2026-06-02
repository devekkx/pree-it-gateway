package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger returns a Zap logger that:
//  1. Writes structured JSON to stdout (captured by Docker / Loki log driver)
//  2. Mirrors every entry to the OTel log pipeline → Collector → Loki
//
// Must be called after observability.Setup() so the global LoggerProvider
// is already registered before the OTel bridge core is created.
func NewLogger(serviceName string) (*zap.Logger, error) {
	// Encoder config
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.MessageKey = "message"
	encCfg.LevelKey = "level"
	encCfg.CallerKey = "caller"

	// Stdout core
	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	// OTel bridge core
	// Sends every log entry to the OTel LoggerProvider → Collector → Loki.
	// trace_id and span_id are attached automatically by the bridge when a
	// span is active in the context, enabling log ↔ trace correlation.
	otelCore := otelzap.NewCore(
		serviceName,
		otelzap.WithLoggerProvider(otellog.GetLoggerProvider()),
	)

	// Combine
	combined := zapcore.NewTee(stdoutCore, otelCore)

	return zap.New(
		combined,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	), nil
}

// WithContext returns a child logger with trace_id and span_id fields
// extracted from the active OTel span in ctx.
// These fields match the derivedFields regex in the Loki datasource config,
// rendering a "View Trace in Tempo" button in Grafana Explore.
func WithContext(ctx context.Context, log *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return log
	}
	return log.With(
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)
}
