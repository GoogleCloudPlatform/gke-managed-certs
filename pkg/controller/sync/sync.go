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

// Package sync contains logic for transitioning ManagedCertificate between states, depending on the state of the cluster.
package sync

import (
	"time"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	networkingv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta1"
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
	ManagedCertificate(id types.CertId) error
}

type syncImpl struct {
	client  networkingv1beta1.NetworkingV1beta1Interface
	config  *config.Config
	lister  mcrtlister.ManagedCertificateLister
	metrics metrics.Metrics
	random  random.Random
	ssl     sslcertificatemanager.SslCertificateManager
	state   state.State
}

func New(client networkingv1beta1.NetworkingV1beta1Interface, config *config.Config, lister mcrtlister.ManagedCertificateLister,
	metrics metrics.Metrics, random random.Random, ssl sslcertificatemanager.SslCertificateManager, state state.State) Sync {
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
	if sslCertificateName, err := s.state.GetSslCertificateName(id); err == nil {
		return sslCertificateName, nil
	}

	sslCertificateName, err := s.random.Name()
	if err != nil {
		return "", err
	}

	glog.Infof("Add to state SslCertificate name %s for ManagedCertificate %s", sslCertificateName, id.String())
	s.state.SetSslCertificateName(id, sslCertificateName)
	return sslCertificateName, nil
}

func (s syncImpl) observeSslCertificateCreationLatencyIfNeeded(sslCertificateName string, id types.CertId, mcrt api.ManagedCertificate) error {
	excludedFromSLO, err := s.state.IsExcludedFromSLO(id)
	if err != nil {
		return err
	}
	if excludedFromSLO {
		glog.Infof("Skipping reporting SslCertificate creation metric, because %s is marked as excluded from SLO calculations.", id.String())
		return nil
	}

	reported, err := s.state.IsSslCertificateCreationReported(id)
	if err != nil {
		return err
	}
	if reported {
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

func (s syncImpl) deleteSslCertificate(mcrt *api.ManagedCertificate, id types.CertId, sslCertificateName string) error {
	glog.Infof("Mark entry for ManagedCertificate %s as soft deleted", id.String())
	if err := s.state.SetSoftDeleted(id); err != nil {
		return err
	}

	glog.Infof("Delete SslCertificate %s for ManagedCertificate %s", sslCertificateName, id.String())
	if err := http.IgnoreNotFound(s.ssl.Delete(sslCertificateName, mcrt)); err != nil {
		return err
	}

	glog.Infof("Remove entry for ManagedCertificate %s from state", id.String())
	s.state.Delete(id)
	return nil
}

func (s syncImpl) ensureSslCertificate(sslCertificateName string, id types.CertId,
	mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {

	exists, err := s.ssl.Exists(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := s.ssl.Create(sslCertificateName, *mcrt); err != nil {
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

	if certificates.Equal(*mcrt, *sslCert) {
		return sslCert, nil
	}

	glog.Infof("ManagedCertificate %v and SslCertificate %v are different", mcrt, sslCert)
	if err := s.deleteSslCertificate(mcrt, id, sslCertificateName); err != nil {
		return nil, err
	}

	return nil, errors.ErrSslCertificateOutOfSyncGotDeleted
}

func (s syncImpl) ManagedCertificate(id types.CertId) error {
	mcrt, err := s.lister.ManagedCertificates(id.Namespace).Get(id.Name)
	if http.IsNotFound(err) {
		sslCertificateName, err := s.state.GetSslCertificateName(id)
		if err == errors.ErrManagedCertificateNotFound {
			return nil
		} else if err != nil {
			return err
		}

		glog.Infof("ManagedCertificate %s already deleted", id.String())
		return s.deleteSslCertificate(nil, id, sslCertificateName)
	} else if err != nil {
		return err
	}

	glog.Infof("Syncing ManagedCertificate %s", id.String())

	sslCertificateName, err := s.ensureSslCertificateName(id)
	if err != nil {
		return err
	}

	sslCert, err := s.ensureSslCertificate(sslCertificateName, id, mcrt)
	if err != nil {
		return err
	}

	if err := certificates.CopyStatus(*sslCert, mcrt, s.config); err != nil {
		return err
	}

	_, err = s.client.ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}
