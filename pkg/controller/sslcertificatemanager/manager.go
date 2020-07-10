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

// Package sslcertificatemanager manipulates SslCertificate objects and communicates GCE API errors with Events.
package sslcertificatemanager

import (
	"context"
	"errors"

	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/klog"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type SslCertificateManager interface {
	Create(ctx context.Context, sslCertificateName string, mcrt apisv1beta2.ManagedCertificate) error
	Delete(ctx context.Context, sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) error
	Exists(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (bool, error)
	Get(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (*compute.SslCertificate, error)
}

type sslCertificateManagerImpl struct {
	event   event.Event
	metrics metrics.Metrics
	ssl     ssl.Ssl
	state   state.State
}

func New(event event.Event, metrics metrics.Metrics, ssl ssl.Ssl, state state.State) SslCertificateManager {
	return sslCertificateManagerImpl{
		event:   event,
		metrics: metrics,
		ssl:     ssl,
		state:   state,
	}
}

// Create creates an SslCertificate object. It generates a TooManyCertificates event if SslCertificate quota
// is exceeded or BackendError event if another generic error occurs. On success it generates a Create event.
func (s sslCertificateManagerImpl) Create(ctx context.Context, sslCertificateName string, mcrt apisv1beta2.ManagedCertificate) error {
	klog.Infof("Creating SslCertificate %s for ManagedCertificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)

	if err := s.ssl.Create(ctx, sslCertificateName, mcrt.Spec.Domains); err != nil {
		var sslErr *ssl.Error
		if errors.As(err, &sslErr) && sslErr.IsQuotaExceeded() {
			s.event.TooManyCertificates(mcrt, err)
			s.metrics.ObserveSslCertificateQuotaError()

			if err := s.state.SetExcludedFromSLO(types.NewCertId(mcrt.Namespace, mcrt.Name)); err != nil {
				return err
			}

			return err
		}

		s.event.BackendError(mcrt, err)
		s.metrics.ObserveSslCertificateBackendError()
		return err
	}

	s.event.Create(mcrt, sslCertificateName)

	klog.Infof("Created SslCertificate %s for ManagedCertificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)
	return nil
}

// Delete deletes an SslCertificate object, existing or not. If a generic error occurs, it generates a BackendError
// event. If the SslCertificate object exists and is successfully deleted, a Delete event is generated.
func (s sslCertificateManagerImpl) Delete(ctx context.Context, sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) error {
	klog.Infof("Deleting SslCertificate %s", sslCertificateName)

	err := s.ssl.Delete(ctx, sslCertificateName)

	if err == nil && mcrt != nil {
		s.event.Delete(*mcrt, sslCertificateName)
	}

	if http.IgnoreNotFound(err) != nil {
		if mcrt != nil {
			s.event.BackendError(*mcrt, err)
			s.metrics.ObserveSslCertificateBackendError()
		}

		return err
	}

	klog.Infof("Deleted SslCertificate %s", sslCertificateName)
	return nil
}

// Exists returns true if an SslCertificate exists, false if it is deleted. Error is not nil if an error has occurred
// and in such case a BackendError event is generated.
func (s sslCertificateManagerImpl) Exists(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (bool, error) {
	exists, err := s.ssl.Exists(sslCertificateName)
	if err != nil {
		if mcrt != nil {
			s.event.BackendError(*mcrt, err)
			s.metrics.ObserveSslCertificateBackendError()
		}
		return false, err
	}

	return exists, nil
}

// Get fetches an SslCertificate object. On error a BackendError event is generated.
func (s sslCertificateManagerImpl) Get(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (*compute.SslCertificate, error) {
	sslCert, err := s.ssl.Get(sslCertificateName)
	if err != nil {
		if mcrt != nil {
			s.event.BackendError(*mcrt, err)
			s.metrics.ObserveSslCertificateBackendError()
		}
		return nil, err
	}

	return sslCert, nil
}
