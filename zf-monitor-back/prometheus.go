package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultPrometheusURL = "http://prometheus-server.monitoring.svc.cluster.local"

const (
	nodeCPUQuery    = `100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m])))`
	nodeMemoryQuery = `100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`
	nodeDiskQuery   = `100 * (1 - node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|overlay"})`
	nodeStatusQuery = `up{service="prometheus-prometheus-node-exporter"}`
)

var prometheusClient = &http.Client{Timeout: 10 * time.Second}

type prometheusQueryResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		Result []struct {
			Value []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type prometheusNodeSummary struct {
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
}

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

func queryPrometheusValue(query string) (float64, error) {
	target, err := url.Parse(strings.TrimRight(prometheusURL(), "/") + "/api/v1/query")
	if err != nil {
		return 0, fmt.Errorf("invalid URL: %w", err)
	}
	targetQuery := target.Query()
	targetQuery.Set("query", query)
	target.RawQuery = targetQuery.Encode()

	response, err := prometheusClient.Get(target.String())
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("Prometheus returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var result prometheusQueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("invalid Prometheus response: %w", err)
	}
	if result.Status != "success" {
		return 0, fmt.Errorf("Prometheus query failed: %s: %s", result.ErrorType, result.Error)
	}
	if len(result.Data.Result) == 0 || len(result.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("Prometheus query returned no value")
	}

	var value string
	if err := json.Unmarshal(result.Data.Result[0].Value[1], &value); err != nil {
		return 0, fmt.Errorf("invalid Prometheus value: %w", err)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid Prometheus value %q: %w", value, err)
	}
	return parsed, nil
}

func handlePrometheusNodeSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cpu, err := queryPrometheusValue(nodeCPUQuery)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus for CPU usage: %v", err), http.StatusBadGateway)
		return
	}
	memory, err := queryPrometheusValue(nodeMemoryQuery)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus for memory usage: %v", err), http.StatusBadGateway)
		return
	}
	disk, err := queryPrometheusValue(nodeDiskQuery)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus for disk usage: %v", err), http.StatusBadGateway)
		return
	}
	up, err := queryPrometheusValue(nodeStatusQuery)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query Prometheus for node status: %v", err), http.StatusBadGateway)
		return
	}

	status := "offline"
	if up == 1 {
		status = "online"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(prometheusNodeSummary{Status: status, CPU: cpu, Memory: memory, Disk: disk})
}
