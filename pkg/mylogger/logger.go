package mylogger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func Info(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	if spanCtx.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	logger.WithOptions(zap.AddCallerSkip(1)).Info(msg, fields...)
}

func Error(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	if spanCtx.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	logger.WithOptions(zap.AddCallerSkip(1)).Error(msg, fields...)
}

func Warn(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	if spanCtx.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	logger.WithOptions(zap.AddCallerSkip(1)).Warn(msg, fields...)
}

func Debug(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	if spanCtx.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	logger.WithOptions(zap.AddCallerSkip(1)).Debug(msg, fields...)
}
