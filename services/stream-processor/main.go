
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "sort"
    "time"
)

func main() {
	rawJSON := `[
		{"service":"api-gateway","status_code":200,"duration_ms":40,"timestamp": "2026-10-21T10:00:01Z"},
		{"service":"api-gateway","status_code":200,"duration_ms":60,"timestamp": "2026-10-21T10:00:03Z"},
		{"service":"api-gateway","status_code":500,"duration_ms":150,"timestamp": "2026-10-21T10:00:01Z"},
		{"service":"order-service","status_code":200,"duration_ms":120,"timestamp": "2026-10-21T10:00:01Z"},
		{"service":"order-service","status_code":500,"duration_ms":400,"timestamp": "2026-10-21T10:00:05Z"},
		{"service":"payment-service","status_code":200,"duration_ms":100,"timestamp": "2026-10-21T10:00:08Z"},
		{"service":"payment-service","status_code":200,"duration_ms":150,"timestamp": "2026-10-21T10:00:05Z"},
		{"service":"payment-service","status_code":500,"duration_ms":850,"timestamp": "2026-10-21T10:00:07Z"},
		{"service":"payment-service","status_code":200,"duration_ms":200,"timestamp": "2026-10-21T10:00:08Z"},
		{"service":"payment-service","status_code":503,"duration_ms":700,"timestamp": "2026-10-21T10:00:01Z"}
	]`

	var events []TelemetryEvent

	if err := json.Unmarshal([]byte(rawJSON), &events); err != nil {
		fmt.Println("Failed to decode telemetry:", err)
		os.Exit(1)
	}

	windowStart, err := time.Parse(
		time.RFC3339,
		"2026-10-21T10:00:00Z",
	)
	if err != nil {
		fmt.Println("Invalid window start:", err)
		os.Exit(1)
	}
	
	window := 10 * time.Second
	
	windowEvents := FilterEventsByWindow(events, windowStart, window)
	metrics := ProcessEvents(windowEvents)

	services := make([]string, 0, len(metrics))
	for service := range metrics {
		services = append(services, service)
	}
	sort.Strings(services)

	fmt.Println("=== Service Health Report ===")

	for _, service := range services {
		m := metrics[service]

		errorRate := float64(m.ServerErrors) /
			float64(m.TotalRequests) * 100

		averageDuration := float64(m.TotalDuration) /
			float64(m.TotalRequests)

		requestRate := float64(m.TotalRequests) / window.Seconds()

		fmt.Printf("\nService: %s\n", service)
		fmt.Printf("Total requests: %d\n", m.TotalRequests)
		fmt.Printf("Server errors: %d\n", m.ServerErrors)
		fmt.Printf("Server error rate: %.2f%%\n", errorRate)
		fmt.Printf("Average duration: %.2f ms\n", averageDuration)
		fmt.Printf("Request rate: %.2f requests/second\n", requestRate)
	}
}
