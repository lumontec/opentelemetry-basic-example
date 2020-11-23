// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Sample contains a program that exports to the OpenCensus service.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Initializes an OTLP exporter, and configures the corresponding trace and
// metric providers.
func initProvider() func() {
	ctx := context.Background()

	collectorAddr := "0.0.0.0:55680"

	exp, err := otlp.NewExporter(
		otlp.WithInsecure(),
		otlp.WithAddress(collectorAddr),
		otlp.WithGRPCDialOption(grpc.WithBlock()), // useful for testing
	)
	handleErr(err, "failed to create exporter")

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("test-service"),
		),
	)
	handleErr(err, "failed to create resource")

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	pusher := push.New(
		basic.New(
			simple.NewWithExactDistribution(),
			exp,
		),
		exp,
		push.WithPeriod(7*time.Second),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(pusher.MeterProvider())
	pusher.Start()

	return func() {
		handleErr(tracerProvider.Shutdown(ctx), "failed to shutdown provider")
		handleErr(exp.Shutdown(ctx), "failed to stop exporter")
		pusher.Stop() // pushes any last exports to the receiver
	}
}

func handleErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}

func main() {
	shutdown := initProvider()
	defer shutdown()

	tracer := otel.Tracer("test-tracer")
	// meter := otel.Meter("test-meter")

	// labels represent additional key-value descriptors that can be bound to a
	// metric observer or recorder.
	// TODO: Use baggage when supported to extact labels from baggage.
	commonLabels := []label.KeyValue{
		label.String("method", "repl"),
		label.String("client", "cli"),
	}

	// Recorder metric example
	// requestLatency := metric.Must(meter).
	// 	NewFloat64ValueRecorder(
	// 		"appdemo/request_latency",
	// 		metric.WithDescription("The latency of requests processed"),
	// 	).Bind(commonLabels...)
	// defer requestLatency.Unbind()

	// TODO: Use a view to just count number of measurements for requestLatency when available.
	// requestCount := metric.Must(meter).
	// 	NewInt64Counter(
	// 		"appdemo/request_counts",
	// 		metric.WithDescription("The number of requests processed"),
	// 	).Bind(commonLabels...)
	// defer requestCount.Unbind()

	// lineLengths := metric.Must(meter).
	// 	NewInt64ValueRecorder(
	// 		"appdemo/line_lengths",
	// 		metric.WithDescription("The lengths of the various lines in"),
	// 	).Bind(commonLabels...)
	// defer lineLengths.Unbind()

	// TODO: Use a view to just count number of measurements for lineLengths when available.
	// lineCounts := metric.Must(meter).
	// 	NewInt64Counter(
	// 		"appdemo/line_counts",
	// 		metric.WithDescription("The counts of the lines in"),
	// 	).Bind(commonLabels...)
	// defer lineCounts.Unbind()

	defaultCtx := baggage.ContextWithValues(context.Background(), commonLabels...)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		f1(defaultCtx, rng, tracer)
	}
}

func f1(ctx context.Context, rng *rand.Rand, tracer trace.Tracer) {
	startTime := time.Now()
	// ctx, span := tracer.Start(defaultCtx, "ExecuteRequest")
	childCtx, span := tracer.Start(ctx, "ExecuteRequest")
	var sleep int64
	switch modulus := time.Now().Unix() % 5; modulus {
	case 0:
		sleep = rng.Int63n(17001)
	case 1:
		sleep = rng.Int63n(8007)
	case 2:
		sleep = rng.Int63n(917)
	case 3:
		sleep = rng.Int63n(87)
	case 4:
		sleep = rng.Int63n(1173)
	}

	time.Sleep(time.Duration(sleep) * time.Millisecond)

	span.End()
	latencyMs := float64(time.Since(startTime)) / 1e6
	nr := int(rng.Int31n(7))
	for i := 0; i < nr; i++ {
		randLineLength := rng.Int63n(999)
		// lineLengths.Record(ctx, randLineLength)
		// lineCounts.Add(ctx, 1)
		fmt.Printf("#%d: LineLength: %dBy\n", i, randLineLength)
	}

	f2(childCtx, rng, tracer)

	// requestLatency.Record(ctx, latencyMs)
	// requestCount.Add(ctx, 1)
	fmt.Printf("Latency: %.3fms\n", latencyMs)
}

func f2(ctx context.Context, rng *rand.Rand, tracer trace.Tracer) {
	startTime := time.Now()
	// ctx, span := tracer.Start(defaultCtx, "ExecuteRequest")
	_, span := tracer.Start(ctx, "ExecuteRequest")
	var sleep int64
	switch modulus := time.Now().Unix() % 5; modulus {
	case 0:
		sleep = rng.Int63n(17001)
	case 1:
		sleep = rng.Int63n(8007)
	case 2:
		sleep = rng.Int63n(917)
	case 3:
		sleep = rng.Int63n(87)
	case 4:
		sleep = rng.Int63n(1173)
	}

	time.Sleep(time.Duration(sleep) * time.Millisecond)

	span.End()
	latencyMs := float64(time.Since(startTime)) / 1e6
	nr := int(rng.Int31n(7))
	for i := 0; i < nr; i++ {
		randLineLength := rng.Int63n(999)
		// lineLengths.Record(ctx, randLineLength)
		// lineCounts.Add(ctx, 1)
		fmt.Printf("#%d: LineLength: %dBy\n", i, randLineLength)
	}

	// requestLatency.Record(ctx, latencyMs)
	// requestCount.Add(ctx, 1)
	fmt.Printf("Latency: %.3fms\n", latencyMs)
}
