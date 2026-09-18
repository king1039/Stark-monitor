package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultPrometheusURL = "http://prometheus-server.monitoring.svc.cluster.local"

var prometheusClient = &http.Client{Timeout: 10 * time.Second}

func prometheusURL() string {
	if value := strings.TrimSpace(os.Getenv("PROMETHEUS_URL")); value != "" {
		return value
	}
	return defaultPrometheusURL
}

func handlePrometheusQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target, err := url.Parse(strings.TrimRight(prometheusURL(), "/") + "/api/v1/query")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus: invalid URL: %v", err), http.StatusBadGateway)
		return
	}
	targetQuery := target.Query()
	targetQuery.Set("query", r.URL.Query().Get("query"))
	target.RawQuery = targetQuery.Encode()

	response, err := prometheusClient.Get(target.String())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus: %v", err), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read Prometheus response: %v", err), http.StatusBadGateway)
		return
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		http.Error(w, fmt.Sprintf("Prometheus returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body))), http.StatusBadGateway)
		return
	}

	contentType := response.Header.Get("Content-Type")
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(body)
}
