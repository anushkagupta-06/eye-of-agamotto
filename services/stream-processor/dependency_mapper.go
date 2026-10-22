package main

// ServiceEdge represents a dependency between two services.
// CallerService calls CalleeService.
type ServiceEdge struct {
	CallerService   string
	CalleeService   string
	TotalCalls      int
	Errors          int
	TotalDurationNs int64
}

// ExtractServiceEdges identifies cross-service calls by matching client spans
// with server spans whose parent span ID refers to the client span.

func ExtractServiceEdges(spans []SpanEvent) map[string]*ServiceEdge {
	edges := make(map[string]*ServiceEdge)

	// Map each client span's key to the server span(s) that are its children.
	serversByParent := make(map[string][]SpanEvent)

	for _, span := range spans {
		if span.TraceID == "" ||
			span.ParentSpanID == "" ||
			span.SpanKind != "SPAN_KIND_SERVER" ||
			span.Service == "" {
			continue
		}

		key := spanKey(span.TraceID, span.ParentSpanID)
		serversByParent[key] = append(serversByParent[key], span)
	}

	for _, client := range spans {
		if client.SpanKind != "SPAN_KIND_CLIENT" ||
			client.TraceID == "" ||
			client.SpanID == "" ||
			client.Service == "" ||
			client.DurationNs < 0 {
			continue
		}

		key := spanKey(client.TraceID, client.SpanID)
		candidates := serversByParent[key]

		for _, server := range candidates {
			if server.Service == client.Service {
				continue
			}

			edgeKey := client.Service + "->" + server.Service
			if edges[edgeKey] == nil {
				edges[edgeKey] = &ServiceEdge{
					CallerService: client.Service,
					CalleeService: server.Service,
				}
			}

			edge := edges[edgeKey]
			edge.TotalCalls++
			edge.TotalDurationNs += client.DurationNs

			statusCode := server.HTTPStatusCode
			if statusCode == 0 {
				statusCode = client.HTTPStatusCode
			}

			if statusCode >= 500 && statusCode <= 599 {
				edge.Errors++
			}
		}
	}

	return edges
}

// spanKey uniquely identifies a span within its trace.
func spanKey(traceID, spanID string) string {
	return traceID + ":" + spanID
}
