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

package e2e

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

const (
	controllerImageTagEnv    = "TAG"
	controllerRegistryEnv    = "REGISTRY"
	gcpServiceAccountFileEnv = "GCP_SERVICE_ACCOUNT_FILE"
	namespace                = "default"
	platformEnv              = "PLATFORM"
)

var clients *client.Clients

func TestMain(m *testing.M) {
	klog.InitFlags(nil)
	flag.Parse()

	ctx := context.Background()

	var err error
	clients, err = client.New(namespace)
	if err != nil {
		klog.Fatalf("Could not create clients: %s", err.Error())
	}

	platform := os.Getenv(platformEnv)
	klog.Infof("platform=%s", platform)
	gke := (strings.ToLower(platform) == "gke")

	sslCertificatesBegin, err := setUp(ctx, clients, gke)
	if err != nil {
		klog.Fatal(err)
	}

	exitCode := m.Run()

	if err := tearDown(ctx, clients, gke, sslCertificatesBegin); err != nil {
		klog.Fatal(err)
	}

	os.Exit(exitCode)
}

func setUp(ctx context.Context, clients *client.Clients, gke bool) ([]*compute.SslCertificate, error) {
	klog.Info("setting up")

	if !gke {
		if err := deployCRD(ctx); err != nil {
			return nil, err
		}

		gcpServiceAccountFileName := os.Getenv(gcpServiceAccountFileEnv)
		gcpServiceAccountJson, err := ioutil.ReadFile(gcpServiceAccountFileName)
		if err != nil {
			return nil, fmt.Errorf("can't read file %s: %w", gcpServiceAccountFileName, err)
		}

		registry := os.Getenv(controllerRegistryEnv)
		tag := os.Getenv(controllerImageTagEnv)
		klog.Infof("Controller image registry=%s, tag=%s", registry, tag)

		if err := deployController(ctx, string(gcpServiceAccountJson), registry, tag); err != nil {
			return nil, err
		}
	}

	if err := clients.ManagedCertificate.DeleteAll(ctx); err != nil {
		return nil, err
	}

	// Try to remove SslCertificate resources that might have been left after previous test runs. Ignore errors.
	if sslCertificates, err := clients.SslCertificate.List(); err != nil {
		return nil, err
	} else if len(sslCertificates) > 0 {
		klog.Infof("Found %d SslCertificate resources, attempting clean up", len(sslCertificates))
		for _, sslCertificate := range sslCertificates {
			klog.Infof("Trying to delete SslCertificate %s", sslCertificate.Name)
			if err := errors.IgnoreNotFound(clients.SslCertificate.Delete(ctx, sslCertificate.Name)); err != nil {
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

func tearDown(ctx context.Context, clients *client.Clients, gke bool, sslCertificatesBegin []*compute.SslCertificate) error {
	klog.Infof("tearing down")

	if err := clients.ManagedCertificate.DeleteAll(ctx); err != nil {
		return err
	}

	if err := utils.Retry(func() error {
		sslCertificatesEnd, err := clients.SslCertificate.List()
		if err != nil {
			return err
		}

		if diff := cmp.Diff(sslCertificatesBegin, sslCertificatesEnd); diff != "" {
			return fmt.Errorf("Waiting for SslCertificates clean up. (-want, +got): %s", diff)
		}

		return nil
	}); err != nil {
		return err
	}

	if !gke {
		name := "managedcertificates.networking.gke.io"
		if err := errors.IgnoreNotFound(clients.CustomResource.Delete(ctx, name, metav1.DeleteOptions{})); err != nil {
			return err
		}
		klog.Infof("Deleted custom resource definition %s", name)

		if err := deleteController(ctx); err != nil {
			return err
		}
	}

	klog.Infof("tear down success")
	return nil
}
