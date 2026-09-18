package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultMetricsAddr = ":9101"

type databaseMetrics struct {
	registry          *prometheus.Registry
	up                *prometheus.GaugeVec
	uptimeSeconds     *prometheus.GaugeVec
	connections       *prometheus.GaugeVec
	maxConnections    *prometheus.GaugeVec
	activeSessions    *prometheus.GaugeVec
	runningRequests   *prometheus.GaugeVec
	databaseCount     *prometheus.GaugeVec
	databaseSizeBytes *prometheus.GaugeVec
}

func newDatabaseMetrics() *databaseMetrics {
	labels := []string{"instance_id", "name", "type"}
	metrics := &databaseMetrics{
		registry:          prometheus.NewRegistry(),
		up:                prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_up", Help: "Whether the database is online."}, labels),
		uptimeSeconds:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_uptime_seconds", Help: "Database uptime in seconds."}, labels),
		connections:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_connections", Help: "Current database connections."}, labels),
		maxConnections:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_max_connections", Help: "Maximum database connections."}, labels),
		activeSessions:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_active_sessions", Help: "Active database sessions."}, labels),
		runningRequests:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_running_requests", Help: "Running database requests."}, labels),
		databaseCount:     prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_count", Help: "Number of databases."}, labels),
		databaseSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "stark_database_size_bytes", Help: "Total database size in bytes."}, labels),
	}
	metrics.registry.MustRegister(
		metrics.up,
		metrics.uptimeSeconds,
		metrics.connections,
		metrics.maxConnections,
		metrics.activeSessions,
		metrics.runningRequests,
		metrics.databaseCount,
		metrics.databaseSizeBytes,
	)
	return metrics
}

func (metrics *databaseMetrics) update(report *DatabaseReport) {
	labels := prometheus.Labels{
		"instance_id": report.InstanceID,
		"name":        report.Name,
		"type":        report.Type,
	}
	up := float64(0)
	if report.Status == "online" {
		up = 1
	}
	metrics.up.With(labels).Set(up)
	metrics.uptimeSeconds.With(labels).Set(report.UptimeSeconds)
	metrics.connections.With(labels).Set(report.Connections)
	metrics.maxConnections.With(labels).Set(report.MaxConnections)
	metrics.activeSessions.With(labels).Set(report.ActiveSessions)
	metrics.runningRequests.With(labels).Set(report.RunningRequests)
	metrics.databaseCount.With(labels).Set(report.DatabaseCount)
	metrics.databaseSizeBytes.With(labels).Set(report.TotalDatabaseSizeMB * 1024 * 1024)
}

func (metrics *databaseMetrics) handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

func metricsAddr() string {
	if value := strings.TrimSpace(os.Getenv("METRICS_ADDR")); value != "" {
		return value
	}
	return defaultMetricsAddr
}
