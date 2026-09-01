// Package metrics owns the Prometheus registry and the metric vectors the rest of the
// control plane writes to. Kept tiny so feature packages just call e.g.
// metrics.RunsTotal.WithLabelValues("succeeded").Inc() without seeing any plumbing.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry is the local registry. We avoid the default registry so test imports and
// shutdowns are clean.
var Registry = prometheus.NewRegistry()

// HTTPRequestsTotal counts REST requests by method + path template + status.
var HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "cc_http_requests_total",
	Help: "Total REST requests handled by the control plane.",
}, []string{"method", "path", "status"})

// HTTPRequestDuration measures REST handler latency.
var HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "cc_http_request_duration_seconds",
	Help:    "REST handler latency in seconds.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "path"})

// AgentsConnected is the number of agents currently holding an open AgentStream.
var AgentsConnected = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cc_agents_connected",
	Help: "Number of agents with an active AgentStream.",
})

// RunsTotal counts run terminations by status (succeeded / failed / timed_out /
// canceled / skipped).
var RunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "cc_runs_total",
	Help: "Total runs by terminal status.",
}, []string{"status"})

// LogSubscribers tracks how many SSE subscribers the broker has, summed across runs.
var LogSubscribers = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "cc_log_subscribers",
	Help: "Current SSE subscribers across all runs.",
})

// RunDuration measures how long runs take, bucketed for the range that actually
// matters here: sub-second health checks through hour-long backups.
var RunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "cc_run_duration_seconds",
	Help:    "Run wall-clock duration in seconds, by terminal status.",
	Buckets: []float64{0.5, 1, 5, 15, 60, 300, 900, 1800, 3600, 7200},
}, []string{"status"})

// RunLogBytes counts log bytes received from agents, including bytes dropped by the
// per-run storage cap. The gap between this and the database size is the cap working.
var RunLogBytes = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "cc_run_log_bytes_total",
	Help: "Total run log bytes received from agents.",
})

// ConnectorOpsTotal counts connector commands by op and outcome. This is the metric
// that answers "is anybody actually managing services through this, and does it work".
var ConnectorOpsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "cc_connector_operations_total",
	Help: "Connector commands by operation and result status.",
}, []string{"op", "status"})

// RetentionDeletedTotal counts rows the pruner removed, by table. A flat line here
// after a retention window is configured means the pruner is not running.
var RetentionDeletedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "cc_retention_deleted_total",
	Help: "Rows deleted by the retention pruner, by table.",
}, []string{"table"})

// NotificationsTotal counts delivery attempts by channel kind and outcome.
var NotificationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "cc_notifications_total",
	Help: "Notification delivery attempts by kind and outcome.",
}, []string{"kind", "outcome"})

func init() {
	Registry.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		AgentsConnected,
		RunsTotal,
		LogSubscribers,
		RunDuration,
		RunLogBytes,
		ConnectorOpsTotal,
		RetentionDeletedTotal,
		NotificationsTotal,
		// Process + Go runtime collectors for free.
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}
