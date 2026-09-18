package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDatabaseMetricsExposeReportValues(t *testing.T) {
	metrics := newDatabaseMetrics()
	metrics.update(&DatabaseReport{
		InstanceID:          "instance-1",
		Name:                "MSSQL",
		Type:                "mssql",
		Status:              "online",
		UptimeSeconds:       12,
		Connections:         3,
		MaxConnections:      10,
		ActiveSessions:      2,
		RunningRequests:     1,
		DatabaseCount:       4,
		TotalDatabaseSizeMB: 2.5,
	})

	recorder := httptest.NewRecorder()
	metrics.handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`stark_database_up{instance_id="instance-1",name="MSSQL",type="mssql"} 1`,
		`stark_database_uptime_seconds{instance_id="instance-1",name="MSSQL",type="mssql"} 12`,
		`stark_database_connections{instance_id="instance-1",name="MSSQL",type="mssql"} 3`,
		`stark_database_size_bytes{instance_id="instance-1",name="MSSQL",type="mssql"} 2.62144e+06`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics output does not contain %q:\n%s", expected, body)
		}
	}
}

func TestDatabaseMetricsSetOfflineStatus(t *testing.T) {
	metrics := newDatabaseMetrics()
	metrics.update(&DatabaseReport{InstanceID: "instance-1", Name: "MSSQL", Type: "mssql", Status: "offline"})

	recorder := httptest.NewRecorder()
	metrics.handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), `stark_database_up{instance_id="instance-1",name="MSSQL",type="mssql"} 0`) {
		t.Fatalf("offline metric not found:\n%s", recorder.Body.String())
	}
}

func TestMetricsAddrUsesDefaultWhenUnset(t *testing.T) {
	previous := os.Getenv("METRICS_ADDR")
	if err := os.Unsetenv("METRICS_ADDR"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("METRICS_ADDR", previous) }()

	if got := metricsAddr(); got != defaultMetricsAddr {
		t.Fatalf("unexpected default metrics address: %s", got)
	}
}

func TestMetricsAddrUsesEnvironmentValue(t *testing.T) {
	previous := os.Getenv("METRICS_ADDR")
	if err := os.Setenv("METRICS_ADDR", "127.0.0.1:19101"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("METRICS_ADDR", previous) }()

	if got := metricsAddr(); got != "127.0.0.1:19101" {
		t.Fatalf("unexpected metrics address: %s", got)
	}
}
