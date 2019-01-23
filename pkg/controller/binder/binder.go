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

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	listers "k8s.io/client-go/listers/extensions/v1beta1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	annotationManagedCertificatesKey = "gke.googleapis.com/managed-certificates"
	annotationPreSharedCertKey       = "ingress.gcp.kubernetes.io/pre-shared-cert"
	separator                        = ","
)

type Binder interface {
	BindCertificates()
}

type binderImpl struct {
	ingressClient v1beta1.IngressesGetter
	ingressLister listers.IngressLister
	state         state.State
}

func New(ingressClient v1beta1.IngressesGetter, ingressLister listers.IngressLister, state state.State) Binder {
	return binderImpl{
		ingressClient: ingressClient,
		ingressLister: ingressLister,
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
		glog.Infof("Attach ManagedCertificates: %v", strings.Join(mcrtsToAttach, ", "))
	}

	if len(sslCertificatesToDetach) > 0 {
		var sslCertsToDetach []string
		for sslCert := range sslCertificatesToDetach {
			sslCertsToDetach = append(sslCertsToDetach, sslCert)
		}
		sort.Strings(sslCertsToDetach)
		glog.Infof("Detach SslCertificates: %v", strings.Join(sslCertsToDetach, ", "))
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

// Ensures certificates are attached to/detached from load balancers depending on the sets of certificates passed
// as arguments.
func (b binderImpl) ensureCertificatesAttached(managedCertificatesToAttach map[types.CertId]string,
	sslCertificatesToDetach map[string]bool) error {

	ingresses, err := b.ingressLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, ingress := range ingresses {
		preSharedCertificates := parse(ingress.Annotations[annotationPreSharedCertKey])
		managedCertificates := parse(ingress.Annotations[annotationManagedCertificatesKey])

		sslCertificatesToBind := make(map[string]bool, 0)
		for item := range preSharedCertificates {
			if _, e := sslCertificatesToDetach[item]; !e {
				sslCertificatesToBind[item] = true
			}
		}

		for id, sslCertificateName := range managedCertificatesToAttach {
			if id.Namespace != ingress.Namespace {
				continue
			}
			if _, e := managedCertificates[id.Name]; e {
				sslCertificatesToBind[sslCertificateName] = true
			}
		}

		var sslCertificates []string
		for sslCertificate := range sslCertificatesToBind {
			sslCertificates = append(sslCertificates, sslCertificate)
		}

		sort.Strings(sslCertificates)
		preSharedCertValue := strings.Join(sslCertificates, separator)

		if preSharedCertValue == ingress.Annotations[annotationPreSharedCertKey] {
			continue
		}

		glog.Infof("Annotation %s on Ingress %s:%s was %s, set to %s", annotationPreSharedCertKey, ingress.Namespace,
			ingress.Name, ingress.Annotations[annotationPreSharedCertKey], preSharedCertValue)

		if ingress.Annotations == nil {
			ingress.Annotations = make(map[string]string, 0)
		}
		ingress.Annotations[annotationPreSharedCertKey] = preSharedCertValue

		if _, err := b.ingressClient.Ingresses(ingress.Namespace).Update(ingress); err != nil {
			return fmt.Errorf("Failed to update Ingress %s:%s: %s", ingress.Namespace, ingress.Name, err.Error())
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
