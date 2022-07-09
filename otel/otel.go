package otel

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/propagation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Otel struct {
	TraceExporter  *otlptrace.Exporter
	MetricExporter *otlpmetric.Exporter
}

func New(ctx context.Context) *Otel {
	traceExporter, err := initTraceExporter(ctx)
	if err != nil {
		otel.Handle(err)
	}

	metricExporter, err := initMetricExporter(ctx)
	if err != nil {
		otel.Handle(err)
	}
	return &Otel{TraceExporter: traceExporter, MetricExporter: metricExporter}
}

func (o *Otel) Shutdown(ctx context.Context) {
	if err := o.TraceExporter.Shutdown(ctx); err != nil {
		otel.Handle(err)
	}
	if err := o.MetricExporter.Shutdown(ctx); err != nil {
		otel.Handle(err)
	}
}

func initTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	conn, err := grpc.DialContext(ctx, "0.0.0.0:4317", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("custom-k8s-api-demo"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return traceExporter, nil
}

func initMetricExporter(ctx context.Context) (*otlpmetric.Exporter, error) {
	client := otlpmetricgrpc.NewClient(otlpmetricgrpc.WithEndpoint("0.0.0.0:4317"), otlpmetricgrpc.WithInsecure())
	exp, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create the collector exporter")
	}

	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithHistogramDistribution(),
			exp,
		),
		controller.WithExporter(exp),
		controller.WithCollectPeriod(time.Second),
	)

	global.SetMeterProvider(pusher)

	if err := pusher.Start(ctx); err != nil {
		return nil, errors.Wrapf(err, "could not start metric controller")
	}
	return exp, nil
}
