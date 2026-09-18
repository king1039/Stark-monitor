package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandlePrometheusQueryReturnsRawResponse(t *testing.T) {
	const responseBody = `{"status":"success","data":{"resultType":"vector"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != `rate(http_requests_total[5m])` {
			t.Fatalf("unexpected query: %s", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()

	previousURL := os.Getenv("PROMETHEUS_URL")
	if err := os.Setenv("PROMETHEUS_URL", server.URL); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("PROMETHEUS_URL", previousURL) }()

	request := httptest.NewRequest(http.MethodGet, "/api/prometheus/query?query=rate%28http_requests_total%5B5m%5D%29", nil)
	recorder := httptest.NewRecorder()
	handlePrometheusQuery(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != responseBody {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePrometheusQueryReturnsBadGatewayOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "query failed", http.StatusBadRequest)
	}))
	defer server.Close()

	previousURL := os.Getenv("PROMETHEUS_URL")
	if err := os.Setenv("PROMETHEUS_URL", server.URL); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("PROMETHEUS_URL", previousURL) }()

	recorder := httptest.NewRecorder()
	handlePrometheusQuery(recorder, httptest.NewRequest(http.MethodGet, "/api/prometheus/query?query=up", nil))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Prometheus returned HTTP 400") {
		t.Fatalf("unexpected error: %s", recorder.Body.String())
	}
}

func TestPrometheusURLUsesDefaultWhenUnset(t *testing.T) {
	previousURL := os.Getenv("PROMETHEUS_URL")
	if err := os.Unsetenv("PROMETHEUS_URL"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("PROMETHEUS_URL", previousURL) }()

	if got := prometheusURL(); got != defaultPrometheusURL {
		t.Fatalf("unexpected default URL: %s", got)
	}
}

func TestHandlePrometheusNodeSummary(t *testing.T) {
	values := map[string]string{
		nodeCPUQuery:    "12.34",
		nodeMemoryQuery: "45.67",
		nodeDiskQuery:   "38.90",
		nodeStatusQuery: "1",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value, ok := values[query]
		if !ok {
			t.Fatalf("unexpected query: %s", query)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":["1","` + value + `"]}]}}`))
	}))
	defer server.Close()

	previousURL := os.Getenv("PROMETHEUS_URL")
	if err := os.Setenv("PROMETHEUS_URL", server.URL); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("PROMETHEUS_URL", previousURL) }()

	recorder := httptest.NewRecorder()
	handlePrometheusNodeSummary(recorder, httptest.NewRequest(http.MethodGet, "/api/prometheus/node/summary", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var summary prometheusNodeSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != "online" || summary.CPU != 12.34 || summary.Memory != 45.67 || summary.Disk != 38.90 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestHandlePrometheusNodeSummaryReturnsBadGatewayOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "query failed", http.StatusBadRequest)
	}))
	defer server.Close()

	previousURL := os.Getenv("PROMETHEUS_URL")
	if err := os.Setenv("PROMETHEUS_URL", server.URL); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("PROMETHEUS_URL", previousURL) }()

	recorder := httptest.NewRecorder()
	handlePrometheusNodeSummary(recorder, httptest.NewRequest(http.MethodGet, "/api/prometheus/node/summary", nil))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "failed to query Prometheus for CPU usage") {
		t.Fatalf("unexpected error: %s", recorder.Body.String())
	}
}
