package kafka

import "github.com/rcrowley/go-metrics"

func init() {
	// to prevent memory leak
	metrics.UseNilMetrics = true
}
