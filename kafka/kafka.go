package kafka

import "github.com/rcrowley/go-metrics"

func init() { //nolint:gochecknoinits // intentional
	// to prevent memory leak
	metrics.UseNilMetrics = true //nolint:reassign // intentionally
}
