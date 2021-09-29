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
	"strings"
	"time"

	computev1 "google.golang.org/api/compute/v1"
	"k8s.io/klog"
)

const (
	codeQuotaExceeded = "QUOTA_EXCEEDED"
	statusDone        = "DONE"
	typeManaged       = "MANAGED"
)

type Error struct {
	operation *computev1.Operation
}

func (e *Error) Error() string {
	var computeErrors []string
	for _, err := range e.operation.Error.Errors {
		computeErrors = append(computeErrors, fmt.Sprintf("(%s: %s)", err.Code, err.Message))
	}

	return fmt.Sprintf("operation %s %s. Status: %s (%d), errors: %s", e.operation.Name,
		e.operation.Status, e.operation.HttpErrorMessage, e.operation.HttpErrorStatusCode,
		strings.Join(computeErrors, ", "))
}

func (e *Error) IsQuotaExceeded() bool {
	for _, err := range e.operation.Error.Errors {
		if err.Code == codeQuotaExceeded {
			return true
		}
	}

	return false
}

// Interface exposes operations for manipulating SslCertificate resources.
type Interface interface {
	// Create creates a new SslCertificate resource.
	Create(ctx context.Context, name string, domains []string) error
	// Delete deletes an SslCertificate resource.
	Delete(ctx context.Context, name string) error
	// Get fetches an SslCertificate resource.
	Get(name string) (*computev1.SslCertificate, error)
	// List fetches all SslCertificate resources.
	List() ([]*computev1.SslCertificate, error)
}

type impl struct {
	service   *computev1.Service
	projectID string
}

func New(client *http.Client, projectID string) (Interface, error) {
	service, err := computev1.New(client)
	if err != nil {
		return nil, err
	}

	return &impl{
		service:   service,
		projectID: projectID,
	}, nil
}

// Create creates a new SslCertificate resource.
func (s impl) Create(ctx context.Context, name string, domains []string) error {
	sslCertificate := &computev1.SslCertificate{
		Managed: &computev1.SslCertificateManagedSslCertificate{
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
func (s impl) Delete(ctx context.Context, name string) error {
	operation, err := s.service.SslCertificates.Delete(s.projectID, name).Do()
	if err != nil {
		return err
	}

	return s.waitFor(ctx, operation.Name)
}

// Get fetches an SslCertificate resource.
func (s impl) Get(name string) (*computev1.SslCertificate, error) {
	return s.service.SslCertificates.Get(s.projectID, name).Do()
}

// List fetches all SslCertificate resources.
func (s impl) List() ([]*computev1.SslCertificate, error) {
	sslCertificates, err := s.service.SslCertificates.List(s.projectID).Do()
	if err != nil {
		return nil, err
	}

	return sslCertificates.Items, nil
}

func (s impl) waitFor(ctx context.Context, operationName string) error {
	for {
		klog.Infof("Wait for operation %s", operationName)
		operation, err := s.service.GlobalOperations.Get(s.projectID, operationName).Do()
		if err != nil {
			return fmt.Errorf("could not get operation %s: %v", operationName, err)
		}

		if operation.Status == statusDone {
			klog.Infof("Operation %s done", operationName)

			if operation.Error == nil {
				return nil
			}

			return &Error{operation: operation}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}
