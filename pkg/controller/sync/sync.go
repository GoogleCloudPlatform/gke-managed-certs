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

// Package sync contains logic for transitioning ManagedCertificate between states, depending on the state of the cluster.
package sync

import (
	"context"
	"time"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/klog"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	clientsetv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1"
	listersv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type Sync interface {
	ManagedCertificate(ctx context.Context, id types.CertId) error
}

type syncImpl struct {
	client  clientsetv1.NetworkingV1Interface
	config  *config.Config
	lister  listersv1.ManagedCertificateLister
	metrics metrics.Metrics
	random  random.Random
	ssl     sslcertificatemanager.SslCertificateManager
	state   state.State
}

func New(client clientsetv1.NetworkingV1Interface, config *config.Config,
	lister listersv1.ManagedCertificateLister, metrics metrics.Metrics,
	random random.Random, ssl sslcertificatemanager.SslCertificateManager,
	state state.State) Sync {

	return syncImpl{
		client:  client,
		config:  config,
		lister:  lister,
		metrics: metrics,
		random:  random,
		ssl:     ssl,
		state:   state,
	}
}

func (s syncImpl) ensureSslCertificateName(id types.CertId) (string, error) {
	if entry, err := s.state.Get(id); err == nil {
		return entry.SslCertificateName, nil
	}

	sslCertificateName, err := s.random.Name()
	if err != nil {
		return "", err
	}

	klog.Infof("Add to state SslCertificate name %s for ManagedCertificate %s",
		sslCertificateName, id.String())

	s.state.Insert(id, sslCertificateName)
	return sslCertificateName, nil
}

func (s syncImpl) observeSslCertificateCreationLatencyIfNeeded(sslCertificateName string,
	id types.CertId, mcrt apisv1.ManagedCertificate) error {

	entry, err := s.state.Get(id)
	if err != nil {
		return err
	}

	if entry.ExcludedFromSLO {
		klog.Infof(`Skipping reporting SslCertificate creation metric,
			because %s is marked as excluded from SLO calculations.`, id.String())

		return nil
	}

	if entry.SslCertificateCreationReported {
		klog.Infof(`Skipping reporting SslCertificate creation metric,
			already reported for %s.`, id.String())

		return nil
	}

	creationTime, err := time.Parse(time.RFC3339, mcrt.CreationTimestamp.Format(time.RFC3339))
	if err != nil {
		return err
	}

	s.metrics.ObserveSslCertificateCreationLatency(creationTime)
	if err := s.state.SetSslCertificateCreationReported(id); err != nil {
		return err
	}

	return nil
}

func (s syncImpl) deleteSslCertificate(ctx context.Context, mcrt *apisv1.ManagedCertificate,
	id types.CertId, sslCertificateName string) error {

	klog.Infof("Mark entry for ManagedCertificate %s as soft deleted", id.String())
	if err := s.state.SetSoftDeleted(id); err != nil {
		return err
	}

	klog.Infof("Delete SslCertificate %s for ManagedCertificate %s",
		sslCertificateName, id.String())

	if err := http.IgnoreNotFound(s.ssl.Delete(ctx, sslCertificateName, mcrt)); err != nil {
		return err
	}

	klog.Infof("Remove entry for ManagedCertificate %s from state", id.String())
	s.state.Delete(id)
	return nil
}

func (s syncImpl) ensureSslCertificate(ctx context.Context, sslCertificateName string,
	id types.CertId, mcrt *apisv1.ManagedCertificate) (*compute.SslCertificate, error) {

	exists, err := s.ssl.Exists(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := s.ssl.Create(ctx, sslCertificateName, *mcrt); err != nil {
			return nil, err
		}

		if err := s.observeSslCertificateCreationLatencyIfNeeded(sslCertificateName, id, *mcrt); err != nil {
			return nil, err
		}
	}

	sslCert, err := s.ssl.Get(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if diff := certificates.Diff(*mcrt, *sslCert); diff != "" {
		klog.Infof(`Certificates out of sync: certificates.Diff(%s, %s): %s,
			ManagedCertificate: %+v, SslCertificate: %+v. Deleting SslCertificate %s`,
			id, sslCert.Name, diff, mcrt, sslCert, sslCert.Name)
		if err := s.deleteSslCertificate(ctx, mcrt, id, sslCertificateName); err != nil {
			return nil, err
		}

		return nil, errors.ErrSslCertificateOutOfSyncGotDeleted
	}

	return sslCert, nil
}

func (s syncImpl) ManagedCertificate(ctx context.Context, id types.CertId) error {
	mcrt, err := s.lister.ManagedCertificates(id.Namespace).Get(id.Name)
	if http.IsNotFound(err) {
		entry, err := s.state.Get(id)
		if err == errors.ErrManagedCertificateNotFound {
			return nil
		} else if err != nil {
			return err
		}

		klog.Infof("ManagedCertificate %s already deleted", id.String())
		return s.deleteSslCertificate(ctx, nil, id, entry.SslCertificateName)
	} else if err != nil {
		return err
	}

	klog.Infof("Syncing ManagedCertificate %s", id.String())

	sslCertificateName, err := s.ensureSslCertificateName(id)
	if err != nil {
		return err
	}

	if entry, err := s.state.Get(id); err != nil {
		return err
	} else if entry.SoftDeleted {
		klog.Infof("ManagedCertificate %s is soft deleted, deleting SslCertificate %s",
			id.String(), sslCertificateName)
		return s.deleteSslCertificate(ctx, mcrt, id, sslCertificateName)
	}

	sslCert, err := s.ensureSslCertificate(ctx, sslCertificateName, id, mcrt)
	if err != nil {
		return err
	}

	if err := certificates.CopyStatus(*sslCert, mcrt, s.config); err != nil {
		return err
	}

	_, err = s.client.ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}
