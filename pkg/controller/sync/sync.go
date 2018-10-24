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
	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
)

type Sync interface {
	ManagedCertificate(namespace, name string) error
	State()
}

type syncImpl struct {
	clientset versioned.Interface
	lister    mcrtlister.ManagedCertificateLister
	ssl       sslcertificatemanager.SslCertificateManager
	state     state.State
}

func New(clientset versioned.Interface, lister mcrtlister.ManagedCertificateLister, ssl sslcertificatemanager.SslCertificateManager, state state.State) Sync {
	return syncImpl{
		clientset: clientset,
		lister:    lister,
		ssl:       ssl,
		state:     state,
	}
}

func (s syncImpl) ensureSslCertificateName(mcrt *api.ManagedCertificate) (string, error) {
	sslCertificateName, exists := s.state.Get(mcrt.Namespace, mcrt.Name)

	if exists {
		return sslCertificateName, nil
	}

	sslCertificateName, err := random.Name()
	if err != nil {
		return "", err
	}

	glog.Infof("Add to state SslCertificate name %s for ManagedCertificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)
	s.state.Put(mcrt.Namespace, mcrt.Name, sslCertificateName)
	return sslCertificateName, nil
}

func (s syncImpl) ensureSslCertificate(sslCertificateName string, mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {
	exists, err := s.ssl.Exists(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := s.ssl.Create(sslCertificateName, *mcrt); err != nil {
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

	glog.Infof("ManagedCertificate %v and SslCertificate %v are different, recreate SslCertificate", mcrt, sslCert)
	if err := http.IgnoreNotFound(s.ssl.Delete(sslCertificateName, mcrt)); err != nil {
		return nil, err
	}
	if err := s.ssl.Create(sslCertificateName, *mcrt); err != nil {
		return nil, err
	}
	sslCert, err = s.ssl.Get(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	return sslCert, nil
}

func (s syncImpl) ManagedCertificate(namespace, name string) error {
	mcrt, err := s.lister.ManagedCertificates(namespace).Get(name)
	if http.IsNotFound(err) {
		if sslCertificateName, exists := s.state.Get(namespace, name); exists {
			glog.Infof("Delete SslCertificate %s, because ManagedCertificate %s:%s already deleted", sslCertificateName, namespace, name)
			err := http.IgnoreNotFound(s.ssl.Delete(sslCertificateName, nil))
			if err != nil {
				return err
			}

			s.state.Delete(namespace, name)
			return nil
		}

		return nil
	} else if err != nil {
		return err
	}

	glog.Infof("Syncing ManagedCertificate %s:%s", mcrt.Namespace, mcrt.Name)

	sslCertificateName, err := s.ensureSslCertificateName(mcrt)
	if err != nil {
		return err
	}

	sslCert, err := s.ensureSslCertificate(sslCertificateName, mcrt)
	if err != nil {
		return err
	}

	if err := certificates.CopyStatus(*sslCert, mcrt); err != nil {
		return err
	}

	_, err = s.clientset.GkeV1alpha1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}

func (s syncImpl) State() {
	s.state.ForeachKey(func(namespace, name string) {
		if err := s.ManagedCertificate(namespace, name); err != nil {
			runtime.HandleError(err)
		}
	})
}
