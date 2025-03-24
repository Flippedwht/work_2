package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// 请求计数器
	RequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	// 请求延迟分布
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}, // 定义延迟分布桶
	}, []string{"method", "path"})

	// LevelDB性能指标
	LevelDBReadOps = promauto.NewCounter(prometheus.CounterOpts{
		Name: "leveldb_read_operations_total",
		Help: "Total LevelDB read operations",
	})
)
