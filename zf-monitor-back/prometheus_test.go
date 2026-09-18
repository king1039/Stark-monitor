package main

import (
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
