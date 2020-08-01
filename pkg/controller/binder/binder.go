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

// Package binder handles binding SslCertificate resources with load balancers via GCE-Ingress's pre-shared-cert annotation.
package binder

import (
	"sort"
	"strings"
	"time"

	apiv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	cgolisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	annotationManagedCertificatesKey = "networking.gke.io/managed-certificates"
	annotationPreSharedCertKey       = "ingress.gcp.kubernetes.io/pre-shared-cert"
	separator                        = ","
)

type Binder interface {
	BindCertificates() error
}

type binderImpl struct {
	eventClient        event.Interface
	ingressClient      v1beta1.IngressesGetter
	ingressLister      cgolisters.IngressLister
	metrics            metrics.Interface
	managedCertificate managedcertificate.Interface
	state              state.Interface
}

func New(eventClient event.Interface, ingressClient v1beta1.IngressesGetter,
	ingressLister cgolisters.IngressLister, managedCertificate managedcertificate.Interface,
	metrics metrics.Interface, state state.Interface) Binder {

	return binderImpl{
		eventClient:        eventClient,
		ingressClient:      ingressClient,
		ingressLister:      ingressLister,
		managedCertificate: managedCertificate,
		metrics:            metrics,
		state:              state,
	}
}

func (b binderImpl) BindCertificates() error {
	managedCertificatesToAttach, sslCertificatesToDetach := b.getCertificatesFromState()

	if len(managedCertificatesToAttach) > 0 {
		var mcrtsToAttach []string
		for id := range managedCertificatesToAttach {
			mcrtsToAttach = append(mcrtsToAttach, id.String())
		}
		sort.Strings(mcrtsToAttach)
	}

	if len(sslCertificatesToDetach) > 0 {
		var sslCertsToDetach []string
		for sslCert := range sslCertificatesToDetach {
			sslCertsToDetach = append(sslCertsToDetach, sslCert)
		}
		sort.Strings(sslCertsToDetach)
	}

	return b.ensureCertificatesAttached(managedCertificatesToAttach, sslCertificatesToDetach)
}

// Based on controller state gets two sets of certificates:
// 1) ManagedCertificates to be attached to load balancers
// 2) SslCertificates to be detached from load balancers.
func (b binderImpl) getCertificatesFromState() (map[types.CertId]string, map[string]bool) {
	managedCertificatesToAttach := make(map[types.CertId]string, 0)
	sslCertificatesToDetach := make(map[string]bool, 0)

	for id, entry := range b.state.List() {
		if entry.SoftDeleted {
			sslCertificatesToDetach[entry.SslCertificateName] = true
		} else {
			managedCertificatesToAttach[id] = entry.SslCertificateName
		}
	}

	return managedCertificatesToAttach, sslCertificatesToDetach
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

// Checks if ManagedCertificate resources attached to this Ingress exist.
// If not, creates an event to communicate the problem to the user.
func (b binderImpl) validateAttachedManagedCertificates(
	ingress *apiv1beta1.Ingress, managedCertificates map[string]bool) error {

	for mcrtName := range managedCertificates {
		_, err := b.managedCertificate.Get(types.NewCertId(ingress.Namespace, mcrtName))

		if err == nil {
			continue
		}

		if http.IsNotFound(err) {
			b.eventClient.MissingCertificate(*ingress, mcrtName)
		}

		return err
	}

	return nil
}

func (b binderImpl) reportManagedCertificatesAttached(ingressNamespace string,
	managedCertificatesToAttach map[types.CertId]string,
	boundManagedCertificates map[string]bool) error {

	for id := range managedCertificatesToAttach {
		if id.Namespace != ingressNamespace {
			continue
		}
		if _, e := boundManagedCertificates[id.Name]; !e {
			continue
		}

		entry, err := b.state.Get(id)
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

		mcrt, err := b.managedCertificate.Get(id)
		if err != nil {
			return err
		}

		creationTime, err := time.Parse(time.RFC3339,
			mcrt.CreationTimestamp.Format(time.RFC3339))
		if err != nil {
			return err
		}

		b.metrics.ObserveSslCertificateBindingLatency(creationTime)

		if err := b.state.SetSslCertificateBindingReported(id); err != nil {
			return err
		}
	}

	return nil
}

// Ensures certificates are attached to/detached from load balancers
// depending on the sets of certificates passed as arguments.
func (b binderImpl) ensureCertificatesAttached(
	managedCertificatesToAttach map[types.CertId]string,
	sslCertificatesToDetach map[string]bool) error {

	ingresses, err := b.ingressLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		boundManagedCertificates := parse(
			ingress.Annotations[annotationManagedCertificatesKey])

		if err := b.validateAttachedManagedCertificates(ingress,
			boundManagedCertificates); err != nil {
			klog.Errorf("validateAttachedManagedCertificates(): %v", err)
			continue
		}

		// Take already bound SslCertificate resources
		sslCertificatesToBind := make(map[string]bool, 0)
		for sslCertificateName := range parse(
			ingress.Annotations[annotationPreSharedCertKey]) {
			if _, e := sslCertificatesToDetach[sslCertificateName]; !e {
				sslCertificatesToBind[sslCertificateName] = true
			}
		}

		// Take SslCertificate resources for bound ManagedCertificate resources
		for id, sslCertificateName := range managedCertificatesToAttach {
			if id.Namespace != ingress.Namespace {
				continue
			}
			if _, e := boundManagedCertificates[id.Name]; e {
				sslCertificatesToBind[sslCertificateName] = true
			}
		}

		preSharedCertValue := buildPreSharedCertAnnotation(sslCertificatesToBind)

		if preSharedCertValue == ingress.Annotations[annotationPreSharedCertKey] {
			continue
		}

		klog.Infof("Annotation %s on Ingress %s:%s was %s, set to %s",
			annotationPreSharedCertKey, ingress.Namespace,
			ingress.Name, ingress.Annotations[annotationPreSharedCertKey],
			preSharedCertValue)

		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string, 0)
		}
		ingress.Annotations[annotationPreSharedCertKey] = preSharedCertValue

		if _, err := b.ingressClient.Ingresses(ingress.Namespace).Update(ingress); err != nil {
			klog.Errorf("Failed to update Ingress %s:%s: %v", ingress.Namespace, ingress.Name, err)
			continue
		}

		if err := b.reportManagedCertificatesAttached(ingress.Namespace,
			managedCertificatesToAttach, boundManagedCertificates); err != nil {
			klog.Errorf("reportManagedCertificatesAttached(): %v", err)
			continue
		}
	}

	return nil
}
