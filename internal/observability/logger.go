package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger returns a Zap logger that:
//  1. Writes human-readable output to stdout (for Docker)
//  2. Mirrors every log entry to the OTel log pipeline → Loki
//  3. Attaches trace_id and span_id to every log entry automatically
func NewLogger(serviceName string) (*zap.Logger, error) {
	// Stdout core — structured JSON
	stdoutCore, err := stdoutZapCore()
	if err != nil {
		return nil, err
	}

	// OTel bridge core — sends to Loki via OTel Collector
	otelCore := otelzap.NewCore(
		serviceName,
		otelzap.WithLoggerProvider(global.GetLoggerProvider()),
	)

	// Combine both cores — logs go to stdout AND Loki
	combined := zapcore.NewTee(stdoutCore, otelCore)

	return zap.New(combined,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	), nil
}

func stdoutZapCore() (zapcore.Core, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.MessageKey = "message"

	enc := zapcore.NewJSONEncoder(cfg)
	ws := zapcore.AddSync(zapcore.Lock(zapcore.NewMultiWriteSyncer()))

	return zapcore.NewCore(enc, zapcore.AddSync(zapcore.Lock(
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
	)), zapcore.InfoLevel), nil
}

// WithContext extracts the active trace span from ctx and returns a logger
// with trace_id and span_id fields attached — these are the keys Loki's
// derivedFields rule matches to link logs → Tempo traces.
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
