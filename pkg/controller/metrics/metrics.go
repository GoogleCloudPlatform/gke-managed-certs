/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package metrics implements metrics for managed certificates.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

const (
	labelStatus   = "status"
	namespace     = "mcrt"
	statusUnknown = "Unknown"
)

type Interface interface {
	Start(address string)
	ObserveManagedCertificatesStatuses(statuses map[string]int)
	ObserveSslCertificateBackendError()
	ObserveSslCertificateQuotaError()
	ObserveSslCertificateBindingLatency(creationTime time.Time)
	ObserveSslCertificateCreationLatency(creationTime time.Time)
}

type impl struct {
	config                          *config.Config
	managedCertificateStatus        *prometheus.GaugeVec
	sslCertificateBackendErrorTotal prometheus.Counter
	sslCertificateQuotaErrorTotal   prometheus.Counter
	sslCertificateBindingLatency    prometheus.Histogram
	sslCertificateCreationLatency   prometheus.Histogram
}

func New(config *config.Config) Interface {
	return impl{
		config: config,
		managedCertificateStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "managedcertificate_status_count",
				Help:      `The number of ManagedCertificate resources partitioned by their statuses`,
			},
			[]string{labelStatus},
		),
		sslCertificateBackendErrorTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sslcertificate_backend_error_total",
				Help: `The number of generic errors occurred
				when performing actions on SslCertificate resources`,
			},
		),
		sslCertificateQuotaErrorTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sslcertificate_quota_error_total",
				Help: `The number of out-of-quota errors occurred
				when performing actions on SslCertificate resources`,
			},
		),
		sslCertificateBindingLatency: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "sslcertificate_binding_latency_seconds",
				Help: `Time elapsed from creating a valid ManagedCertificate resource to binding a first
				corresponding SslCertificate resource with Ingress via annotation pre-shared-cert`,
				Buckets: prometheus.ExponentialBuckets(1.0, 1.3, 10),
			},
		),
		sslCertificateCreationLatency: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "sslcertificate_creation_latency_seconds",
				Help: `Time elapsed from creating a valid ManagedCertificate resource
				to creating a first corresponding SslCertificate resource`,
				Buckets: prometheus.ExponentialBuckets(1.0, 1.3, 10),
			},
		),
	}
}

// Start exposes Prometheus metrics on given address
func (m impl) Start(address string) {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(address, nil)
	klog.Fatalf("Failed to expose metrics: %v", err)
}

// ObserveManagedCertificatesStatuses accepts a mapping from ManagedCertificate certificate status to number of occurences of this status among ManagedCertificate resources and records the data as a metric.
func (m impl) ObserveManagedCertificatesStatuses(statuses map[string]int) {
	for mcrtStatus, occurences := range statuses {
		label := statusUnknown
		for _, v := range m.config.CertificateStatus.Certificate {
			if mcrtStatus == v {
				label = mcrtStatus
				break
			}
		}

		m.managedCertificateStatus.With(prometheus.Labels{labelStatus: label}).Set(float64(occurences))
	}
}

// ObserveSslCertificateBackendError observes an error when performing action on SslCertificate resource.
func (m impl) ObserveSslCertificateBackendError() {
	m.sslCertificateBackendErrorTotal.Inc()
}

// ObserveSslCertificateQuotaError observes an out-of-quota error when performing action on SslCertificate resource.
func (m impl) ObserveSslCertificateQuotaError() {
	m.sslCertificateQuotaErrorTotal.Inc()
}

// ObserveSslCertificateBindingLatency observes the time it took to bind an SslCertficate resource with Ingress after
// a valid ManagedCertficate resource was created.
func (m impl) ObserveSslCertificateBindingLatency(creationTime time.Time) {
	diff := time.Now().UTC().Sub(creationTime).Seconds()
	m.sslCertificateBindingLatency.Observe(diff)
}

// ObserveSslCertificateCreationLatency observes the time it took to create an SslCertficate resource after a valid
// ManagedCertficate resource was created.
func (m impl) ObserveSslCertificateCreationLatency(creationTime time.Time) {
	diff := time.Now().UTC().Sub(creationTime).Seconds()
	m.sslCertificateCreationLatency.Observe(diff)
}
