package middleware

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// === 轻量 Prometheus 指标（无外部依赖） ===

var (
	metricRequests   atomic.Int64
	metricErrors     atomic.Int64
	metricLatencySum atomic.Int64 // 累计延迟(微秒)
	metricStartTime  = time.Now()
)

// MetricsCollector 请求指标采集中间件
func MetricsCollector() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start).Microseconds()

		metricRequests.Add(1)
		metricLatencySum.Add(latency)
		if c.Writer.Status() >= 400 {
			metricErrors.Add(1)
		}
	}
}

// MetricsSnapshot 供告警/监控接口读取的指标快照
func MetricsSnapshot() (requests, errors int64, avgLatencyMicros float64, uptime time.Duration) {
	reqs := metricRequests.Load()
	errs := metricErrors.Load()
	if reqs > 0 {
		avgLatencyMicros = float64(metricLatencySum.Load()) / float64(reqs)
	}
	return reqs, errs, avgLatencyMicros, time.Since(metricStartTime)
}

// MetricsHandler Prometheus 格式指标端点 /metrics
func MetricsHandler(c *gin.Context) {
	reqs := metricRequests.Load()
	errs := metricErrors.Load()
	latSum := metricLatencySum.Load()
	uptime := time.Since(metricStartTime).Seconds()

	var avgLatency float64
	if reqs > 0 {
		avgLatency = float64(latSum) / float64(reqs)
	}

	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(c.Writer, "# HELP lab_teaching_uptime_seconds Service uptime in seconds.\n")
	fmt.Fprintf(c.Writer, "# TYPE lab_teaching_uptime_seconds gauge\n")
	fmt.Fprintf(c.Writer, "lab_teaching_uptime_seconds %.2f\n", uptime)
	fmt.Fprintf(c.Writer, "# HELP lab_teaching_http_requests_total Total HTTP requests.\n")
	fmt.Fprintf(c.Writer, "# TYPE lab_teaching_http_requests_total counter\n")
	fmt.Fprintf(c.Writer, "lab_teaching_http_requests_total %d\n", reqs)
	fmt.Fprintf(c.Writer, "# HELP lab_teaching_http_errors_total Total HTTP errors (4xx+5xx).\n")
	fmt.Fprintf(c.Writer, "# TYPE lab_teaching_http_errors_total counter\n")
	fmt.Fprintf(c.Writer, "lab_teaching_http_errors_total %d\n", errs)
	fmt.Fprintf(c.Writer, "# HELP lab_teaching_http_latency_avg_microseconds Average request latency.\n")
	fmt.Fprintf(c.Writer, "# TYPE lab_teaching_http_latency_avg_microseconds gauge\n")
	fmt.Fprintf(c.Writer, "lab_teaching_http_latency_avg_microseconds %.1f\n", avgLatency)
}

// HealthDetailHandler 详细健康检查（含DB状态）
func HealthDetailHandler(dbOK func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "up"
		if !dbOK() {
			dbStatus = "down"
		}
		status := http.StatusOK
		if dbStatus == "down" {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"status":  "ok",
			"service": "lab-teaching",
			"time":    time.Now().Format(time.RFC3339),
			"uptime":  time.Since(metricStartTime).String(),
			"db":      dbStatus,
			"metrics": gin.H{
				"requests_total": metricRequests.Load(),
				"errors_total":   metricErrors.Load(),
			},
		})
	}
}
