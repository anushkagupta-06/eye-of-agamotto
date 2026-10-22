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

// ExtractServiceEdges identifies calls by matching client spans
// with the server spans they invoke in the same trace.
func ExtractServiceEdges(spans []SpanEvent) map[string]*ServiceEdge {
	edges := make(map[string]*ServiceEdge)

	// Index spans by trace ID and span ID for quick parent lookup.
	spanByID := make(map[string]SpanEvent)

	for _, span := range spans {
		if span.TraceID == "" || span.SpanID == "" {
			continue
		}

		key := span.TraceID + ":" + span.SpanID
		spanByID[key] = span
	}

	for _, client := range spans {
		if client.SpanKind != "SPAN_KIND_CLIENT" ||
			client.TraceID == "" ||
			client.SpanID == "" ||
			client.Service == "" {
			continue
		}

		// Find the server span whose parent is this client span.
		var server *SpanEvent

		for _, candidate := range spans {
			if candidate.TraceID == client.TraceID &&
				candidate.ParentSpanID == client.SpanID &&
				candidate.SpanKind == "SPAN_KIND_SERVER" &&
				candidate.Service != "" {
				match := candidate
				server = &match
				break
			}
		}

		// Without a matching server span, we cannot confidently
		// identify the destination service using this method.
		if server == nil || server.Service == client.Service {
			continue
		}

		key := client.Service + "->" + server.Service

		if edges[key] == nil {
			edges[key] = &ServiceEdge{
				CallerService: client.Service,
				CalleeService: server.Service,
			}
		}

		edge := edges[key]
		edge.TotalCalls++
		edge.TotalDurationNs += client.DurationNs

		// Prefer the server's HTTP status when available.
		statusCode := server.HTTPStatusCode
		if statusCode == 0 {
			statusCode = client.HTTPStatusCode
		}

		if statusCode >= 500 {
			edge.Errors++
		}
	}

	return edges
}
