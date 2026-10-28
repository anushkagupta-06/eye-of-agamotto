package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QueryAPIClient delegates calls to the Eye of Agamotto QueryAPI HTTP service.
type QueryAPIClient struct {
	baseURL    string
	httpClient *http.Client
	tenantID   string
}

// NewQueryAPIClient creates a new QueryAPI HTTP client.
func NewQueryAPIClient(baseURL, tenantID string) *QueryAPIClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8082"
	}
	if tenantID == "" {
		tenantID = "demo"
	}
	return &QueryAPIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		tenantID:   tenantID,
	}
}

// SearchTraces queries matching traces.
func (c *QueryAPIClient) SearchTraces(ctx context.Context, service string, minDurationNs uint64, status string, limit int) (string, error) {
	u, _ := url.Parse(c.baseURL + "/api/v1/traces")
	q := u.Query()
	q.Set("tenant", c.tenantID)
	if service != "" {
		q.Set("service", service)
	}
	if minDurationNs > 0 {
		q.Set("minDuration", strconv.FormatUint(minDurationNs, 10))
	}
	if status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	} else {
		q.Set("limit", "20")
	}
	u.RawQuery = q.Encode()

	return c.doGet(ctx, u.String())
}

// GetTrace fetches a complete trace DAG by traceId.
func (c *QueryAPIClient) GetTrace(ctx context.Context, traceID string) (string, error) {
	if traceID == "" {
		return "", fmt.Errorf("traceId is required")
	}
	urlStr := fmt.Sprintf("%s/api/v1/traces/%s?tenant=%s", c.baseURL, url.PathEscape(traceID), url.QueryEscape(c.tenantID))
	return c.doGet(ctx, urlStr)
}

// QueryLogs searches logs by token, severity, or service.
func (c *QueryAPIClient) QueryLogs(ctx context.Context, query, timeRange, service, severity string, limit int) (string, error) {
	u, _ := url.Parse(c.baseURL + "/api/v1/logs")
	q := u.Query()
	q.Set("tenant", c.tenantID)
	if query != "" {
		q.Set("query", query)
	}
	if service != "" {
		q.Set("service", service)
	}
	if severity != "" {
		q.Set("severity", severity)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	} else {
		q.Set("limit", "50")
	}

	rangeDur := 15 * time.Minute
	if timeRange != "" {
		if d, err := time.ParseDuration(timeRange); err == nil {
			rangeDur = d
		}
	}
	q.Set("from", time.Now().Add(-rangeDur).Format(time.RFC3339))
	q.Set("to", time.Now().Format(time.RFC3339))

	u.RawQuery = q.Encode()
	return c.doGet(ctx, u.String())
}

// GetServiceTopology retrieves the caller-to-callee dependency graph.
func (c *QueryAPIClient) GetServiceTopology(ctx context.Context, timeRange string) (string, error) {
	u, _ := url.Parse(c.baseURL + "/api/v1/topology")
	q := u.Query()
	q.Set("tenant", c.tenantID)

	rangeDur := 15 * time.Minute
	if timeRange != "" {
		if d, err := time.ParseDuration(timeRange); err == nil {
			rangeDur = d
		}
	}
	q.Set("from", time.Now().Add(-rangeDur).Format(time.RFC3339))
	q.Set("to", time.Now().Format(time.RFC3339))
	u.RawQuery = q.Encode()

	return c.doGet(ctx, u.String())
}

// ListAnomalies lists detected statistical anomalies.
func (c *QueryAPIClient) ListAnomalies(ctx context.Context, timeRange string, limit int) (string, error) {
	u, _ := url.Parse(c.baseURL + "/api/v1/anomalies")
	q := u.Query()
	q.Set("tenant", c.tenantID)
	if timeRange != "" {
		q.Set("range", timeRange)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	return c.doGet(ctx, u.String())
}

// ExplainAnomaly fetches detailed diagnostic evidence for an anomaly ID.
func (c *QueryAPIClient) ExplainAnomaly(ctx context.Context, anomalyID string) (string, error) {
	if anomalyID == "" {
		return "", fmt.Errorf("anomaly id is required")
	}
	urlStr := fmt.Sprintf("%s/api/v1/anomalies/%s?tenant=%s", c.baseURL, url.PathEscape(anomalyID), url.QueryEscape(c.tenantID))
	return c.doGet(ctx, urlStr)
}

func (c *QueryAPIClient) doGet(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-Id", c.tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call query-api at %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("query-api returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Format JSON nicely for LLM consumption
	var prettyJSON any
	if err := json.Unmarshal(bodyBytes, &prettyJSON); err == nil {
		formatted, errFmt := json.MarshalIndent(prettyJSON, "", "  ")
		if errFmt == nil {
			return string(formatted), nil
		}
	}

	return string(bodyBytes), nil
}
