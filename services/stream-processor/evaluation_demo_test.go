
package main

import (
	"fmt"
	"sort"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	otlptrace "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func demoAttribute(key string, value *commonv1.AnyValue) *commonv1.KeyValue {
	return &commonv1.KeyValue{Key: key, Value: value}
}

func demoString(value string) *commonv1.AnyValue {
	return &commonv1.AnyValue{
		Value: &commonv1.AnyValue_StringValue{StringValue: value},
	}
}

func demoInt(value int64) *commonv1.AnyValue {
	return &commonv1.AnyValue{
		Value: &commonv1.AnyValue_IntValue{IntValue: value},
	}
}

func TestEvaluationDemo(t *testing.T) {
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

	fmt.Println("\n========== 1. DECODER DEMO ==========")

	request := &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*otlptrace.ResourceSpans{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						demoAttribute("service.name", demoString("checkout")),
					},
				},
				ScopeSpans: []*otlptrace.ScopeSpans{
					{
						Spans: []*otlptrace.Span{
							{
								TraceId:           []byte{1, 2, 3, 4},
								SpanId:            []byte{10, 11},
								Name:              "POST /checkout",
								Kind:              otlptrace.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: uint64(start.UnixNano()),
								EndTimeUnixNano:   uint64(start.Add(120 * time.Millisecond).UnixNano()),
								Attributes: []*commonv1.KeyValue{
									demoAttribute("http.response.status_code", demoInt(200)),
								},
							},
						},
					},
				},
			},
		},
	}

	payload, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeOTLPTraces(payload)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range decoded {
		fmt.Printf(
			"Service=%s | TraceID=%s | SpanID=%s | Name=%s\n",
			s.Service, s.TraceID, s.SpanID, s.SpanName,
		)
		fmt.Printf(
			"Kind=%s | Duration=%d ns (%.2f ms) | HTTP Status=%d\n",
			s.SpanKind, s.DurationNs,
			float64(s.DurationNs)/1e6, s.HTTPStatusCode,
		)
	}

	fmt.Println("\n========== 2. RED METRICS DEMO ==========")

	// All durations are known so the calculations are easy to verify.
	spans := []SpanEvent{
		{Service: "checkout", TraceID: "t1", SpanID: "s1",
			SpanName: "request-1", SpanKind: "SPAN_KIND_SERVER",
			StartTime: start, DurationNs: int64(100 * time.Millisecond),
			HTTPStatusCode: 200},

		{Service: "checkout", TraceID: "t2", SpanID: "s2",
			SpanName: "request-2", SpanKind: "SPAN_KIND_SERVER",
			StartTime: start.Add(time.Second), DurationNs: int64(200 * time.Millisecond),
			HTTPStatusCode: 500},

		{Service: "checkout", TraceID: "t3", SpanID: "s3",
			SpanName: "request-3", SpanKind: "SPAN_KIND_SERVER",
			StartTime: start.Add(2 * time.Second), DurationNs: int64(150 * time.Millisecond),
			HTTPStatusCode: 200},

		{Service: "checkout", TraceID: "t4", SpanID: "s4",
			SpanName: "request-4", SpanKind: "SPAN_KIND_SERVER",
			StartTime: start.Add(3 * time.Second), DurationNs: int64(300 * time.Millisecond),
			HTTPStatusCode: 200},

		// A client span is deliberately included; RED metrics should ignore it.
		{Service: "checkout", TraceID: "t5", SpanID: "client1",
			SpanKind: "SPAN_KIND_CLIENT", StartTime: start,
			DurationNs: int64(900 * time.Millisecond), HTTPStatusCode: 200},
	}

	metrics := ProcessSpanWindow(spans, start, time.Minute)

	m := metrics["checkout"]
	if m == nil {
		t.Fatal("expected checkout metrics")
	}

	fmt.Printf("Service: %s\n", m.Service)
	fmt.Printf("Requests: %d / 60 sec = %.4f requests/sec\n",
		m.TotalRequests, m.RequestRate)
	fmt.Printf("Errors: %d; Error rate: %.2f%%\n", m.Errors, m.ErrorRate)
	fmt.Printf("Average duration: %.2f ms\n", m.AverageDurationMs)
	fmt.Printf("p50=%.2f ms | p95=%.2f ms | p99=%.2f ms\n",
		m.P50Ms, m.P95Ms, m.P99Ms)

	fmt.Println("\nCalculation check:")
	fmt.Println("Durations: 100, 150, 200, 300 ms")
	fmt.Println("Average = (100 + 150 + 200 + 300) / 4 = 187.5 ms")
	fmt.Println("Errors = 1 out of 4 server requests = 25%")
	fmt.Println("Sorted durations + nearest-rank method: p50=150, p95=300, p99=300 ms")

	fmt.Println("\n========== 3. DEPENDENCY GRAPH DEMO ==========")

	dependencySpans := []SpanEvent{
		// Checkout client call; payment server span is its child.
		{Service: "checkout", TraceID: "trace-A", SpanID: "client-payment",
			SpanKind: "SPAN_KIND_CLIENT", DurationNs: int64(120 * time.Millisecond),
			HTTPStatusCode: 200},

		{Service: "payment", TraceID: "trace-A", SpanID: "payment-server",
			ParentSpanID: "client-payment", SpanKind: "SPAN_KIND_SERVER",
			DurationNs: int64(100 * time.Millisecond), HTTPStatusCode: 200},

		// Payment client call; inventory server span is its child.
		{Service: "payment", TraceID: "trace-A", SpanID: "client-inventory",
			SpanKind: "SPAN_KIND_CLIENT", DurationNs: int64(60 * time.Millisecond),
			HTTPStatusCode: 500},

		{Service: "inventory", TraceID: "trace-A", SpanID: "inventory-server",
			ParentSpanID: "client-inventory", SpanKind: "SPAN_KIND_SERVER",
			DurationNs: int64(50 * time.Millisecond), HTTPStatusCode: 500},
	}

	edges := ExtractServiceEdges(dependencySpans)

	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		edge := edges[key]
		fmt.Printf("%s -> %s | Calls=%d | Errors=%d | Client duration total=%.2f ms\n",
			edge.CallerService, edge.CalleeService,
			edge.TotalCalls, edge.Errors,
			float64(edge.TotalDurationNs)/1e6)
	}

	fmt.Println("\nDemo completed.")
}
