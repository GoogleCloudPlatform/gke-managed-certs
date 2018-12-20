/*
Copyright 2018 Google LLC

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

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client-go metrics registration

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

const (
	labelStatus   = "status"
	namespace     = "mcrt"
	statusUnknown = "Unknown"
)

type Metrics interface {
	Start(address string)
	ObserveManagedCertificatesStatuses(statuses map[string]int)
	ObserveSslCertificateCreationLatency(creationTime time.Time)
}

type metricsImpl struct {
	config                        *config.Config
	managedCertificateStatus      *prometheus.GaugeVec
	sslCertificateCreationLatency prometheus.Histogram
}

func New(config *config.Config) Metrics {
	return metricsImpl{
		config: config,
		managedCertificateStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "managedcertificate_status_count",
				Help:      `The number of ManagedCertificate resources partitioned by their statuses`,
			},
			[]string{labelStatus},
		),
		sslCertificateCreationLatency: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "sslcertificate_creation_latency_seconds",
				Help: `Time elapsed from creating a valid ManagedCertificate resource
				to creating a first corresponding SslCertificate resource`,
			},
		),
	}
}

// Start exposes Prometheus metrics on given address
func (m metricsImpl) Start(address string) {
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(address, nil)
	glog.Fatalf("Failed to expose metrics: %s", err.Error())
}

// ObserveManagedCertificatesStatuses accepts a mapping from ManagedCertificate certificate status to number of occurences of this status among ManagedCertificate resources and records the data as a metric.
func (m metricsImpl) ObserveManagedCertificatesStatuses(statuses map[string]int) {
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

// ObserveSslCertificateCreationLatency observes the time it took to create an SslCertficate resource after a valid ManagedCertficate resource was created.
func (m metricsImpl) ObserveSslCertificateCreationLatency(creationTime time.Time) {
	diff := time.Now().UTC().Sub(creationTime).Seconds()
	m.sslCertificateCreationLatency.Observe(diff)
}
