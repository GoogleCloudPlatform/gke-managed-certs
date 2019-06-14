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

// Package binder handles binding SslCertificate resources with load balancers via GCE-Ingress's pre-shared-cert annotation.
package binder

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	listers "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/klog"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	annotationManagedCertificatesKey = "networking.gke.io/managed-certificates"
	annotationPreSharedCertKey       = "ingress.gcp.kubernetes.io/pre-shared-cert"
	separator                        = ","
)

type Binder interface {
	BindCertificates()
}

type binderImpl struct {
	ingressClient v1beta1.IngressesGetter
	ingressLister listers.IngressLister
	metrics       metrics.Metrics
	mcrtLister    api.ManagedCertificateLister
	state         state.State
}

func New(ingressClient v1beta1.IngressesGetter, ingressLister listers.IngressLister, mcrtLister api.ManagedCertificateLister,
	metrics metrics.Metrics, state state.State) Binder {

	return binderImpl{
		ingressClient: ingressClient,
		ingressLister: ingressLister,
		mcrtLister:    mcrtLister,
		metrics:       metrics,
		state:         state,
	}
}

func (b binderImpl) BindCertificates() {
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

	if err := b.ensureCertificatesAttached(managedCertificatesToAttach, sslCertificatesToDetach); err != nil {
		runtime.HandleError(err)
	}
}

// Based on controller state gets two sets of certificates: ManagedCertificates to be attached to load balancers
// and SslCertificates to be detached from load balancers.
func (b binderImpl) getCertificatesFromState() (map[types.CertId]string, map[string]bool) {
	managedCertificatesToAttach := make(map[types.CertId]string, 0)
	sslCertificatesToDetach := make(map[string]bool, 0)

	b.state.ForeachKey(func(id types.CertId) {
		sslCertificateName, err := b.state.GetSslCertificateName(id)
		if err != nil {
			runtime.HandleError(err)
			return
		}

		if softDeleted, err := b.state.IsSoftDeleted(id); err != nil {
			runtime.HandleError(err)
			return
		} else if softDeleted {
			sslCertificatesToDetach[sslCertificateName] = true
		} else {
			managedCertificatesToAttach[id] = sslCertificateName
		}
	})

	return managedCertificatesToAttach, sslCertificatesToDetach
}

// Builds the value of pre-shared-cert annotation out of a set of SslCertificate resources' names
func buildPreSharedCertAnnotation(sslCertificates map[string]bool) string {
	var result []string
	for sslCertificate := range sslCertificates {
		result = append(result, sslCertificate)
	}

	sort.Strings(result)
	return strings.Join(result, separator)
}

// Ensures certificates are attached to/detached from load balancers depending on the sets of certificates passed
// as arguments.
func (b binderImpl) ensureCertificatesAttached(managedCertificatesToAttach map[types.CertId]string,
	sslCertificatesToDetach map[string]bool) error {

	ingresses, err := b.ingressLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		boundManagedCertificates := parse(ingress.Annotations[annotationManagedCertificatesKey])

		// Take already bound SslCertificate resources
		sslCertificatesToBind := make(map[string]bool, 0)
		for sslCertificateName := range parse(ingress.Annotations[annotationPreSharedCertKey]) {
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

		klog.Infof("Annotation %s on Ingress %s:%s was %s, set to %s", annotationPreSharedCertKey, ingress.Namespace,
			ingress.Name, ingress.Annotations[annotationPreSharedCertKey], preSharedCertValue)

		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string, 0)
		}
		ingress.Annotations[annotationPreSharedCertKey] = preSharedCertValue

		if _, err := b.ingressClient.Ingresses(ingress.Namespace).Update(ingress); err != nil {
			return fmt.Errorf("Failed to update Ingress %s:%s: %s", ingress.Namespace, ingress.Name, err.Error())
		}

		for id := range managedCertificatesToAttach {
			if id.Namespace != ingress.Namespace {
				continue
			}
			if _, e := boundManagedCertificates[id.Name]; e {
				excludedFromSLO, err := b.state.IsExcludedFromSLO(id)
				if err != nil {
					return err
				}
				if excludedFromSLO {
					klog.Infof("Skipping reporting SslCertificate binding metric, because %s is marked as excluded from SLO calculations.", id.String())
					return nil
				}

				reported, err := b.state.IsSslCertificateBindingReported(id)
				if err != nil {
					return err
				}

				if reported {
					return nil
				}

				mcrt, err := b.mcrtLister.ManagedCertificates(id.Namespace).Get(id.Name)
				if err != nil {
					return err
				}

				creationTime, err := time.Parse(time.RFC3339, mcrt.CreationTimestamp.Format(time.RFC3339))
				if err != nil {
					return err
				}

				b.metrics.ObserveSslCertificateBindingLatency(creationTime)

				if err := b.state.SetSslCertificateBindingReported(id); err != nil {
					return err
				}
			}
		}
	}

	return nil
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
