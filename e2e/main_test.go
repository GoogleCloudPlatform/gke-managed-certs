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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

const (
	namespace = "default"
)

var clients *client.Clients

func TestMain(m *testing.M) {
	flag.Parse()

	var err error
	clients, err = client.New(namespace)
	if err != nil {
		glog.Fatalf("Could not create clients: %s", err.Error())
	}

	sslCertificatesBegin, err := setUp(clients)
	if err != nil {
		glog.Fatal(err)
	}

	exitCode := m.Run()

	if err := tearDown(clients, sslCertificatesBegin); err != nil {
		glog.Fatal(err)
	}

	os.Exit(exitCode)
}

func setUp(clients *client.Clients) ([]*compute.SslCertificate, error) {
	glog.Info("setting up")

	if err := clients.ManagedCertificate.DeleteAll(); err != nil {
		return nil, err
	}

	sslCertificatesBegin, err := clients.SslCertificate.List()
	if err != nil {
		return nil, err
	}

	glog.Info("set up success")
	return sslCertificatesBegin, nil
}

func tearDown(clients *client.Clients, sslCertificatesBegin []*compute.SslCertificate) error {
	glog.Infof("tearing down")

	if err := clients.ManagedCertificate.DeleteAll(); err != nil {
		return err
	}

	if err := utils.Retry(func() error {
		sslCertificatesEnd, err := clients.SslCertificate.List()
		if err != nil {
			return err
		}

		if added, removed, equal := diff(sslCertificatesBegin, sslCertificatesEnd); !equal {
			return fmt.Errorf("Waiting for SslCertificates clean up. + %v - %v, want both empty", added, removed)
		}

		return nil
	}); err != nil {
		return err
	}

	glog.Infof("tear down success")
	return nil
}

func diff(begin, end []*compute.SslCertificate) ([]string, []string, bool) {
	var added, removed []string

	for _, b := range begin {
		found := false

		for _, e := range end {
			if b.Name == e.Name {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, b.Name)
		}
	}

	for _, e := range end {
		found := false

		for _, b := range begin {
			if e.Name == b.Name {
				found = true
				break
			}
		}

		if !found {
			added = append(added, e.Name)
		}
	}

	return added, removed, len(added) == 0 && len(removed) == 0
}
