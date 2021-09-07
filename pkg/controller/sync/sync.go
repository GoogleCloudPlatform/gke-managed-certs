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

// Package sync contains logic for synchronizing Ingress and ManagedCertificate resources
// with user intent.
package sync

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	computev1 "google.golang.org/api/compute/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/patch"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	separator = ","
)

// Interface provides operations for synchronizing resources with user intent.
type Interface interface {
	// ManagedCertificate synchronizes ManagedCertificate resources
	// with user intent.
	ManagedCertificate(ctx context.Context, id types.Id) error
	// Ingress synchronizes ManagedCertificate resources
	// with user intent.
	Ingress(ctx context.Context, id types.Id) error
}

type impl struct {
	config             *config.Config
	event              event.Interface
	ingress            ingress.Interface
	managedCertificate managedcertificate.Interface
	metrics            metrics.Interface
	random             random.Interface
	ssl                sslcertificatemanager.Interface
	state              state.Interface
}

func New(config *config.Config, event event.Interface, ingress ingress.Interface,
	managedCertificate managedcertificate.Interface, metrics metrics.Interface,
	random random.Interface, ssl sslcertificatemanager.Interface,
	state state.Interface) Interface {

	return impl{
		config:             config,
		event:              event,
		ingress:            ingress,
		managedCertificate: managedCertificate,
		metrics:            metrics,
		random:             random,
		ssl:                ssl,
		state:              state,
	}
}

// Splits given comma separated string into a set of non-empty items
func parse(annotation string) map[string]bool {
	result := make(map[string]bool, 0)
	for _, item := range strings.Split(annotation, separator) {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

// Returns:
// 1. a set of SslCertificate resources that should be attached to Ingress
// via annotation pre-shared-cert.
// 2. a slice of ManagedCertificate ids that are attached to Ingress
// via annotation managed-certificates.
// 3. an error on failure.
func (s impl) getCertificatesToAttach(ingress *netv1.Ingress) (map[string]bool, []types.Id, error) {
	// If a ManagedCertificate attached to Ingress does not exist, add an event to Ingress
	// and return an error.
	boundManagedCertificates := parse(ingress.Annotations[config.AnnotationManagedCertificatesKey])
	for mcrtName := range boundManagedCertificates {
		id := types.NewId(ingress.Namespace, mcrtName)
		_, err := s.managedCertificate.Get(id)

		if err == nil {
			continue
		}

		if errors.IsNotFound(err) {
			s.event.MissingCertificate(*ingress, mcrtName)
		}

		return nil, nil, fmt.Errorf("managedCertificate.Get(%s): %w", id.String(), err)
	}

	// Take already bound SslCertificate resources.
	sslCertificates := make(map[string]bool, 0)
	for sslCertificateName := range parse(ingress.Annotations[config.AnnotationPreSharedCertKey]) {
		sslCertificates[sslCertificateName] = true
	}

	// Slice of ManagedCertificate ids that are attached to Ingress via annotation
	// managed-certificates.
	var managedCertificates []types.Id

	for id, entry := range s.state.List() {
		if id.Namespace != ingress.Namespace {
			continue
		}

		if entry.SoftDeleted {
			delete(sslCertificates, entry.SslCertificateName)
		} else if _, e := boundManagedCertificates[id.Name]; e {
			sslCertificates[entry.SslCertificateName] = true
			managedCertificates = append(managedCertificates, id)
		}
	}

	return sslCertificates, managedCertificates, nil
}

// Builds the value of pre-shared-cert annotation
// out of a set of SslCertificate resources' names.
func buildPreSharedCertAnnotation(sslCertificates map[string]bool) string {
	var result []string
	for sslCertificate := range sslCertificates {
		result = append(result, sslCertificate)
	}

	sort.Strings(result)
	return strings.Join(result, separator)
}

func (s impl) reportManagedCertificatesAttached(ctx context.Context, managedCertificates []types.Id) error {
	for _, id := range managedCertificates {
		entry, err := s.state.Get(id)
		if err != nil {
			return err
		}

		if entry.ExcludedFromSLO {
			klog.Infof("Skipping reporting SslCertificate binding metric: %s is marked as excluded from SLO calculations.", id.String())
			continue
		}

		if entry.SslCertificateBindingReported {
			klog.Infof("Skipping reporting SslCertificate binding metric: already reported for %s.", id.String())
			continue
		}

		mcrt, err := s.managedCertificate.Get(id)
		if err != nil {
			return err
		}

		creationTime, err := time.Parse(time.RFC3339, mcrt.CreationTimestamp.Format(time.RFC3339))
		if err != nil {
			return err
		}

		s.metrics.ObserveSslCertificateBindingLatency(creationTime)

		if err := s.state.SetSslCertificateBindingReported(ctx, id); err != nil {
			return err
		}
	}

	return nil
}

func (s impl) Ingress(ctx context.Context, id types.Id) error {
	originalIngress, err := s.ingress.Get(id)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	klog.Infof("Syncing Ingress %s", id.String())

	sslCertificates, managedCertificates, err := s.getCertificatesToAttach(originalIngress)
	if err != nil {
		return fmt.Errorf("getCertificatesToAttach(): %w", err)
	}

	preSharedCertValue := buildPreSharedCertAnnotation(sslCertificates)

	if preSharedCertValue == originalIngress.Annotations[config.AnnotationPreSharedCertKey] {
		return nil
	}

	klog.Infof("Annotation %s on Ingress %s was %s, set to %s",
		config.AnnotationPreSharedCertKey, id.String(),
		originalIngress.Annotations[config.AnnotationPreSharedCertKey], preSharedCertValue)

	modifiedIngress := originalIngress.DeepCopy()

	if modifiedIngress.Annotations == nil {
		modifiedIngress.Annotations = make(map[string]string, 0)
	}
	modifiedIngress.Annotations[config.AnnotationPreSharedCertKey] = preSharedCertValue

	patchBytes, modified, err := patch.CreateMergePatch(originalIngress, modifiedIngress)
	if err != nil {
		return fmt.Errorf("patch.CreateMergePatch(): %w", err)
	}

	if modified {
		err = s.ingress.Patch(ctx, id, patchBytes)
		if err != nil {
			return fmt.Errorf("s.ingress.Patch(): %w", err)
		}
	}

	if err := s.reportManagedCertificatesAttached(ctx, managedCertificates); err != nil {
		return fmt.Errorf("reportManagedCertificatesAttached(): %w", err)
	}

	return nil
}

func (s impl) insertSslCertificateName(ctx context.Context, id types.Id) (string, error) {
	if entry, err := s.state.Get(id); err == nil {
		return entry.SslCertificateName, nil
	}

	sslCertificateName, err := s.random.Name()
	if err != nil {
		return "", err
	}

	klog.Infof("Add to state SslCertificate name %s for ManagedCertificate %s",
		sslCertificateName, id.String())

	s.state.Insert(ctx, id, sslCertificateName)
	return sslCertificateName, nil
}

func (s impl) observeSslCertificateCreationLatency(ctx context.Context, sslCertificateName string,
	id types.Id, managedCertificate v1.ManagedCertificate) error {

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

	creationTime, err := time.Parse(time.RFC3339,
		managedCertificate.CreationTimestamp.Format(time.RFC3339))
	if err != nil {
		return err
	}

	s.metrics.ObserveSslCertificateCreationLatency(creationTime)
	if err := s.state.SetSslCertificateCreationReported(ctx, id); err != nil {
		return err
	}

	return nil
}

func (s impl) deleteSslCertificate(ctx context.Context,
	managedCertificate *v1.ManagedCertificate,
	id types.Id, sslCertificateName string) error {

	klog.Infof("Mark entry for ManagedCertificate %s as soft deleted", id.String())
	if err := s.state.SetSoftDeleted(ctx, id); err != nil {
		return err
	}

	klog.Infof("Delete SslCertificate %s for ManagedCertificate %s",
		sslCertificateName, id.String())

	if err := errors.IgnoreNotFound(s.ssl.Delete(ctx, sslCertificateName, managedCertificate)); err != nil {
		return err
	}

	klog.Infof("Remove entry for ManagedCertificate %s from state", id.String())
	s.state.Delete(ctx, id)
	return nil
}

func (s impl) createSslCertificate(ctx context.Context, sslCertificateName string,
	id types.Id, managedCertificate *v1.ManagedCertificate) (*computev1.SslCertificate, error) {

	exists, err := s.ssl.Exists(sslCertificateName, managedCertificate)
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := s.ssl.Create(ctx, sslCertificateName, *managedCertificate); err != nil {
			return nil, err
		}

		if err := s.observeSslCertificateCreationLatency(ctx, sslCertificateName,
			id, *managedCertificate); err != nil {
			return nil, err
		}
	}

	sslCert, err := s.ssl.Get(sslCertificateName, managedCertificate)
	if err != nil {
		return nil, err
	}

	if diff := certificates.Diff(*managedCertificate, *sslCert); diff != "" {
		klog.Infof(`Certificates out of sync: certificates.Diff(%s, %s): %s,
			ManagedCertificate: %+v, SslCertificate: %+v. Deleting SslCertificate %s`,
			id, sslCert.Name, diff, managedCertificate, sslCert, sslCert.Name)
		if err := s.deleteSslCertificate(ctx, managedCertificate, id, sslCertificateName); err != nil {
			return nil, err
		}

		return nil, errors.OutOfSync
	}

	return sslCert, nil
}

func (s impl) ManagedCertificate(ctx context.Context, id types.Id) error {
	originalMcrt, err := s.managedCertificate.Get(id)
	if errors.IsNotFound(err) {
		entry, err := s.state.Get(id)
		if errors.IsNotFound(err) {
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

	sslCertificateName, err := s.insertSslCertificateName(ctx, id)
	if err != nil {
		return err
	}

	modifiedMcrt := originalMcrt.DeepCopy()

	if entry, err := s.state.Get(id); err != nil {
		return err
	} else if entry.SoftDeleted {
		klog.Infof("ManagedCertificate %s is soft deleted, deleting SslCertificate %s",
			id.String(), sslCertificateName)
		return s.deleteSslCertificate(ctx, modifiedMcrt, id, sslCertificateName)
	}

	sslCert, err := s.createSslCertificate(ctx, sslCertificateName, id, modifiedMcrt)
	if err != nil {
		return err
	}

	if err := certificates.CopyStatus(*sslCert, modifiedMcrt, s.config); err != nil {
		return err
	}

	patchBytes, modified, err := patch.CreateMergePatch(originalMcrt, modifiedMcrt)
	if err != nil {
		return fmt.Errorf("patch.CreateMergePatch(): %w", err)
	}

	if modified {
		err = s.managedCertificate.Patch(ctx, id, patchBytes)
	}
	return err
}
