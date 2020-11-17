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

// Package sslcertificatemanager manipulates SslCertificate resources
// and communicates GCE API errors with Events.
package sslcertificatemanager

import (
	"context"
	"errors"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/klog"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	utilserrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Interface provides operations for manipulating SslCertificate resources
// and communicates GCE API errors with Events.
type Interface interface {
	// Create creates an SslCertificate object. It generates a TooManyCertificates event
	// if SslCertificate quota is exceeded or BackendError event if another
	// generic error occurs. On success it generates a Create event.
	Create(ctx context.Context, sslCertificateName string, managedCertificate apisv1.ManagedCertificate) error
	// Delete deletes an SslCertificate object, existing or not. If a generic error occurs,
	// it generates a BackendError event. If the SslCertificate object exists
	// and is successfully deleted, a Delete event is generated.
	Delete(ctx context.Context, sslCertificateName string, managedCertificate *apisv1.ManagedCertificate) error
	// Exists returns true if an SslCertificate exists, false if it is deleted.
	// Error is not nil if an error has occurred and in such case
	// a BackendError event is generated.
	Exists(sslCertificateName string, managedCertificate *apisv1.ManagedCertificate) (bool, error)
	// Get fetches an SslCertificate object. On error a BackendError event is generated.
	Get(sslCertificateName string, managedCertificate *apisv1.ManagedCertificate) (*compute.SslCertificate, error)
}

type impl struct {
	event   event.Interface
	metrics metrics.Interface
	ssl     ssl.Interface
	state   state.Interface
}

func New(event event.Interface, metrics metrics.Interface, ssl ssl.Interface, state state.Interface) Interface {
	return impl{
		event:   event,
		metrics: metrics,
		ssl:     ssl,
		state:   state,
	}
}

// Create creates an SslCertificate object. It generates a TooManyCertificates event
// if SslCertificate quota is exceeded or BackendError event if another
// generic error occurs. On success it generates a Create event.
func (s impl) Create(ctx context.Context, sslCertificateName string,
	managedCertificate apisv1.ManagedCertificate) error {

	klog.Infof("Creating SslCertificate %s for ManagedCertificate %s:%s",
		sslCertificateName, managedCertificate.Namespace, managedCertificate.Name)

	if err := s.ssl.Create(ctx, sslCertificateName, managedCertificate.Spec.Domains); err != nil {
		var sslErr *ssl.Error
		if errors.As(err, &sslErr) && sslErr.IsQuotaExceeded() {
			s.event.TooManyCertificates(managedCertificate, err)
			s.metrics.ObserveSslCertificateQuotaError()

			id := types.NewId(managedCertificate.Namespace, managedCertificate.Name)
			if err := s.state.SetExcludedFromSLO(ctx, id); err != nil {
				return err
			}

			return err
		}

		s.event.BackendError(managedCertificate, err)
		s.metrics.ObserveSslCertificateBackendError()
		return err
	}

	s.event.Create(managedCertificate, sslCertificateName)

	klog.Infof("Created SslCertificate %s for ManagedCertificate %s:%s",
		sslCertificateName, managedCertificate.Namespace, managedCertificate.Name)

	return nil
}

// Delete deletes an SslCertificate object, existing or not. If a generic error occurs,
// it generates a BackendError event. If the SslCertificate object exists
// and is successfully deleted, a Delete event is generated.
func (s impl) Delete(ctx context.Context, sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) error {

	klog.Infof("Deleting SslCertificate %s", sslCertificateName)

	err := s.ssl.Delete(ctx, sslCertificateName)

	if err == nil && managedCertificate != nil {
		s.event.Delete(*managedCertificate, sslCertificateName)
	}

	if utilserrors.IgnoreNotFound(err) != nil {
		s.metrics.ObserveSslCertificateBackendError()

		if managedCertificate != nil {
			s.event.BackendError(*managedCertificate, err)
		}

		return err
	}

	klog.Infof("Deleted SslCertificate %s", sslCertificateName)
	return nil
}

// Exists returns true if an SslCertificate exists, false if it is deleted.
// Error is not nil if an error has occurred and in such case
// a BackendError event is generated.
func (s impl) Exists(sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) (bool, error) {

	exists, err := s.ssl.Exists(sslCertificateName)
	if err != nil {
		s.metrics.ObserveSslCertificateBackendError()

		if managedCertificate != nil {
			s.event.BackendError(*managedCertificate, err)
		}

		return false, err
	}

	return exists, nil
}

// Get fetches an SslCertificate object. On error a BackendError event is generated.
func (s impl) Get(sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) (*compute.SslCertificate, error) {

	sslCert, err := s.ssl.Get(sslCertificateName)
	if err != nil {
		s.metrics.ObserveSslCertificateBackendError()

		if managedCertificate != nil {
			s.event.BackendError(*managedCertificate, err)
		}

		return nil, err
	}

	return sslCert, nil
}
