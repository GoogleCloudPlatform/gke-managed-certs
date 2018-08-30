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

package sslcertificate

import (
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.alpha"
	gcfg "gopkg.in/gcfg.v1"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

const httpTimeout = 30 * time.Second

type SslClient struct {
	service   *compute.Service
	projectId string
}

func NewClient(cloudConfig string) (*SslClient, error) {
	tokenSource := google.ComputeTokenSource("") // [review]: var tokenSource ...

	if len(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) > 0 {
		tokenSource, err := google.DefaultTokenSource(oauth2.NoContext, compute.ComputeScope)
		if err != nil {
			return nil, err
		}

		glog.V(1).Infof("In a GCP cluster, using TokenSource: %v", tokenSource)
	} else if cloudConfig != "" { // [review]: shouldn't cloudConfig take precedance?
		config, err := os.Open(cloudConfig)
		if err != nil {
			return nil, fmt.Errorf("Could not open cloud provider configuration %s: %#v", cloudConfig, err)
		}
		defer config.Close()

		var cfg gce.ConfigFile
		if err := gcfg.ReadInto(&cfg, config); err != nil {
			return nil, fmt.Errorf("Could not read config %v", err)
		}

		tokenSource = gce.NewAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody) // [review]: you are creating a new var here...
		glog.V(1).Infof("In a GKE cluster, using TokenSource: %v", tokenSource)
	} else {
		// [review]: tokenSource = google.ComputeTokenSource("")
		glog.V(1).Infof("Using default TokenSource: %v", tokenSource)
	}

	projectId, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch project id: %v", err)
	}

	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client.Timeout = httpTimeout

	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	return &SslClient{
		service:   service,
		projectId: projectId,
	}, nil
}

func (c *SslClient) Delete(name string) error {
	_, err := c.service.SslCertificates.Delete(c.projectId, name).Do()
	return err
}

func (c *SslClient) Get(name string) (*compute.SslCertificate, error) {
	return c.service.SslCertificates.Get(c.projectId, name).Do()
}

func (c *SslClient) Insert(sslCertificateName string, domains []string) error {
	sslCertificate := &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: domains,
		},
		Name: sslCertificateName,
		Type: "MANAGED",
	}

	_, err := c.service.SslCertificates.Insert(c.projectId, sslCertificate).Do()
	if err != nil { // [review]: why wrap the error?
		return fmt.Errorf("Failed to insert SslCertificate %v, err: %v", sslCertificate, err)
	}

	return nil
}

func (c *SslClient) List() (*compute.SslCertificateList, error) {
	return c.service.SslCertificates.List(c.projectId).Do()
}
