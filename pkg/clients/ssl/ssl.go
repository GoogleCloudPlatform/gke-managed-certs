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

// Package ssl provides operations for manipulating SslCertificate GCE resources.
package ssl

import (
	"context"
	"fmt"
	"net/http"
	"time"

	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/klog"

	utilshttp "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	statusDone  = "DONE"
	typeManaged = "MANAGED"
)

type Ssl interface {
	Create(ctx context.Context, name string, domains []string) error
	Delete(ctx context.Context, name string) error
	Exists(name string) (bool, error)
	Get(name string) (*compute.SslCertificate, error)
	List() ([]*compute.SslCertificate, error)
}

type sslImpl struct {
	service   *compute.Service
	projectID string
}

func New(client *http.Client, projectID string) (Ssl, error) {
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
func (s sslImpl) Create(ctx context.Context, name string, domains []string) error {
	sslCertificate := &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: domains,
		},
		Name: name,
		Type: typeManaged,
	}

	operation, err := s.service.SslCertificates.Insert(s.projectID, sslCertificate).Do()
	if err != nil {
		return err
	}

	return s.waitFor(ctx, operation.Name)
}

// Delete deletes an SslCertificate resource.
func (s sslImpl) Delete(ctx context.Context, name string) error {
	operation, err := s.service.SslCertificates.Delete(s.projectID, name).Do()
	if err != nil {
		return err
	}

	return s.waitFor(ctx, operation.Name)
}

// Exists returns true if an SslCertificate exists, false if it is deleted. Error is not nil if an error has occurred.
func (s sslImpl) Exists(name string) (bool, error) {
	_, err := s.Get(name)
	if err == nil {
		return true, nil
	}

	if utilshttp.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

// Get fetches an SslCertificate resource.
func (s sslImpl) Get(name string) (*compute.SslCertificate, error) {
	return s.service.SslCertificates.Get(s.projectID, name).Do()
}

// List fetches all SslCertificate resources.
func (s sslImpl) List() ([]*compute.SslCertificate, error) {
	sslCertificates, err := s.service.SslCertificates.List(s.projectID).Do()
	if err != nil {
		return nil, err
	}

	return sslCertificates.Items, nil
}

func (s sslImpl) waitFor(ctx context.Context, operationName string) error {
	for {
		klog.Infof("Wait for operation %s", operationName)
		operation, err := s.service.GlobalOperations.Get(s.projectID, operationName).Do()
		if err != nil {
			return fmt.Errorf("could not get operation %s: %s", operationName, err.Error())
		}

		if operation.Status == statusDone {
			klog.Infof("Operation %s done, %+v", operationName, operation)
			if operation.HttpErrorMessage == "" {
				return nil
			}

			return fmt.Errorf("operation %s failed: %s", operationName, operation.HttpErrorMessage)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}
