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

// Package ssl provides operations for manipulating SslCertificate GCE resources.
package ssl

import (
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	gcfg "gopkg.in/gcfg.v1"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	httpTimeout = 30 * time.Second
	typeManaged = "MANAGED"
)

type Ssl interface {
	Create(name string, domains []string) error
	Delete(name string) error
	Exists(name string) (bool, error)
	Get(name string) (*compute.SslCertificate, error)
}

type sslImpl struct {
	service   *compute.Service
	projectID string
}

func getTokenSource(gceConfigFilePath string) (oauth2.TokenSource, error) {
	if gceConfigFilePath != "" {
		glog.V(1).Info("In a GKE cluster")

		config, err := os.Open(gceConfigFilePath)
		if err != nil {
			return nil, fmt.Errorf("Could not open cloud provider configuration %s: %v", gceConfigFilePath, err)
		}
		defer config.Close()

		var cfg gce.ConfigFile
		if err := gcfg.ReadInto(&cfg, config); err != nil {
			return nil, fmt.Errorf("Could not read config %v", err)
		}
		glog.Infof("Using GCE provider config %+v", cfg)

		return gce.NewAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody), nil
	} else if len(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) > 0 {
		glog.V(1).Info("In a GCP cluster")
		return google.DefaultTokenSource(oauth2.NoContext, compute.ComputeScope)
	} else {
		glog.V(1).Info("Using default TokenSource")
		return google.ComputeTokenSource(""), nil
	}
}

func New(gceConfigFilePath string) (Ssl, error) {
	tokenSource, err := getTokenSource(gceConfigFilePath)
	if err != nil {
		return nil, err
	}

	glog.V(1).Infof("TokenSource: %v", tokenSource)

	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch project id: %v", err)
	}

	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client.Timeout = httpTimeout

	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	return &sslImpl{
		service:   service,
		projectID: projectID,
	}, nil
}

// Create creates a new SslCertificate resource.
func (s sslImpl) Create(name string, domains []string) error {
	sslCertificate := &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: domains,
		},
		Name: name,
		Type: typeManaged,
	}

	_, err := s.service.SslCertificates.Insert(s.projectID, sslCertificate).Do()
	return err
}

// Delete deletes an SslCertificate resource.
func (s sslImpl) Delete(name string) error {
	_, err := s.service.SslCertificates.Delete(s.projectID, name).Do()
	return err
}

// Exists returns true if an SslCertificate exists, false if it is deleted. Error is not nil if an error has occurred.
func (s sslImpl) Exists(name string) (bool, error) {
	_, err := s.Get(name)
	if err == nil {
		return true, nil
	}

	if http.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

// Get fetches an SslCertificate resource.
func (s sslImpl) Get(name string) (*compute.SslCertificate, error) {
	return s.service.SslCertificates.Get(s.projectID, name).Do()
}
