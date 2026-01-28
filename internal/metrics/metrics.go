/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// BurstsTotal is the total number of burst events processed.
	BurstsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burstscaler_bursts_total",
			Help: "Total number of burst events processed",
		},
		[]string{"name", "namespace"},
	)

	// SourceMessagesTotal is the total number of source messages consumed.
	SourceMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burstscaler_source_messages_total",
			Help: "Total number of source messages consumed",
		},
		[]string{"name", "namespace"},
	)

	// ConsumerStatus indicates whether a consumer is running (1) or stopped (0).
	ConsumerStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "burstscaler_consumer_status",
			Help: "Consumer status (1=running, 0=stopped)",
		},
		[]string{"name", "namespace"},
	)

	// ReconcileTotal is the total number of reconcile operations.
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burstscaler_reconcile_total",
			Help: "Total number of reconcile operations",
		},
		[]string{"name", "namespace", "result"},
	)

	// ReconcileDuration is the duration of reconcile operations.
	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "burstscaler_reconcile_duration_seconds",
			Help:    "Duration of reconcile operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "namespace"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		BurstsTotal,
		SourceMessagesTotal,
		ConsumerStatus,
		ReconcileTotal,
		ReconcileDuration,
	)
}
