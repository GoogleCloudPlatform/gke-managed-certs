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
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	compute "google.golang.org/api/compute/v0.beta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	utilshttp "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	controllerImageTagEnv = "TAG"
	namespace             = "default"
	platformEnv           = "PLATFORM"
)

var clients *client.Clients

func TestMain(m *testing.M) {
	klog.InitFlags(nil)
	flag.Parse()

	var err error
	clients, err = client.New(namespace)
	if err != nil {
		klog.Fatalf("Could not create clients: %s", err.Error())
	}

	platform := os.Getenv(platformEnv)
	klog.Infof("platform=%s", platform)
	gke := (strings.ToLower(platform) == "gke")

	sslCertificatesBegin, err := setUp(clients, gke)
	if err != nil {
		klog.Fatal(err)
	}

	exitCode := m.Run()

	if err := tearDown(clients, gke, sslCertificatesBegin); err != nil {
		klog.Fatal(err)
	}

	os.Exit(exitCode)
}

func setUp(clients *client.Clients, gke bool) ([]*compute.SslCertificate, error) {
	klog.Info("setting up")

	if !gke {
		if err := deployCRD(); err != nil {
			return nil, err
		}

		tag := os.Getenv(controllerImageTagEnv)
		klog.Infof("Controller image tag=%s", tag)
		if err := deployController(tag); err != nil {
			return nil, err
		}
	}

	if err := clients.ManagedCertificate.DeleteAll(); err != nil {
		return nil, err
	}

	// Try to remove SslCertificate resources that might have been left after previous test runs. Ignore errors.
	if sslCertificates, err := clients.SslCertificate.List(); err != nil {
		return nil, err
	} else if len(sslCertificates) > 0 {
		klog.Infof("Found %d SslCertificate resources, attempting clean up", len(sslCertificates))
		for _, sslCertificate := range sslCertificates {
			klog.Infof("Trying to delete SslCertificate %s", sslCertificate.Name)
			if err := clients.SslCertificate.Delete(context.Background(), sslCertificate.Name); err != nil {
				klog.Warningf("Failed to delete %s, err: %v", sslCertificate.Name, err)
			}
		}
	}

	sslCertificatesBegin, err := clients.SslCertificate.List()
	if err != nil {
		return nil, err
	}

	klog.Info("set up success")
	return sslCertificatesBegin, nil
}

func tearDown(clients *client.Clients, gke bool, sslCertificatesBegin []*compute.SslCertificate) error {
	klog.Infof("tearing down")

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

	if !gke {
		name := "managedcertificates.networking.gke.io"
		if err := utilshttp.IgnoreNotFound(clients.CustomResource.Delete(name, &metav1.DeleteOptions{})); err != nil {
			return err
		}
		klog.Infof("Deleted custom resource definition %s", name)

		if err := deleteController(); err != nil {
			return err
		}
	}

	klog.Infof("tear down success")
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
