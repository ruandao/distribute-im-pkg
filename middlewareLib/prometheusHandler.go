package middlewarelib

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 定义指标
var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
)

func init() {
	// 注册指标处理程序
	http.Handle("/metrics", promhttp.Handler())
}

var MetricsMiddleware HandF = func(ctx context.Context, w http.ResponseWriter, r *http.Request, nextF NextF) {
	// 记录请求开始时间
	start := time.Now()

	nextF(ctx, w, r)

	endTime := time.Now()
	// 记录请求指标
	duration := endTime.Sub(start)
	status := "200"

	requestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
	requestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration.Seconds())
}
