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

// Package event provides operations for manipulating Event objects.
package event

import (
	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
)

const (
	component                 = "managed-certificate-controller"
	namespace                 = ""
	reasonBackendError        = "BackendError"
	reasonCreate              = "Create"
	reasonDelete              = "Delete"
	reasonMissingCertificate  = "MissingCertificate"
	reasonTooManyCertificates = "TooManyCertificates"
)

type Event interface {
	BackendError(mcrt apisv1.ManagedCertificate, err error)
	Create(mcrt apisv1.ManagedCertificate, sslCertificateName string)
	Delete(mcrt apisv1.ManagedCertificate, sslCertificateName string)
	MissingCertificate(ingress extv1beta1.Ingress, mcrtName string)
	TooManyCertificates(mcrt apisv1.ManagedCertificate, err error)
}

type eventImpl struct {
	recorder record.EventRecorder
}

// New creates an event recorder to send custom events to Kubernetes.
func New(client kubernetes.Interface) (Event, error) {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(klog.V(4).Infof)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{
		Interface: corev1.New(client.CoreV1().RESTClient()).Events(namespace),
	})

	eventsScheme := runtime.NewScheme()
	if err := apisv1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}
	if err := extv1beta1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}

	return &eventImpl{
		recorder: broadcaster.NewRecorder(eventsScheme,
			v1.EventSource{Component: component}),
	}, nil
}

// BackendError creates an event when a transient error occurrs when calling GCP API.
func (e eventImpl) BackendError(mcrt apisv1.ManagedCertificate, err error) {
	e.recorder.Event(&mcrt, v1.EventTypeWarning, reasonBackendError, err.Error())
}

// Create creates an event when an SslCertificate associated with ManagedCertificate is created.
func (e eventImpl) Create(mcrt apisv1.ManagedCertificate, sslCertificateName string) {
	e.recorder.Eventf(&mcrt, v1.EventTypeNormal, reasonCreate,
		"Create SslCertificate %s", sslCertificateName)
}

// Delete creates an event when an SslCertificate associated with ManagedCertificate is deleted.
func (e eventImpl) Delete(mcrt apisv1.ManagedCertificate, sslCertificateName string) {
	e.recorder.Eventf(&mcrt, v1.EventTypeNormal, reasonDelete,
		"Delete SslCertificate %s", sslCertificateName)
}

// MissingCertificate creates an event when a ManagedCertificate attached to Ingress
// is not found.
func (e eventImpl) MissingCertificate(ingress extv1beta1.Ingress, mcrtName string) {
	e.recorder.Eventf(&ingress, v1.EventTypeWarning, reasonMissingCertificate,
		"ManagedCertificate %s:%s missing", ingress.Namespace, mcrtName)
}

// TooManyCertificates creates an event when quota for maximum
// number of SslCertificates per GCP project is exceeded.
func (e eventImpl) TooManyCertificates(mcrt apisv1.ManagedCertificate, err error) {
	e.recorder.Event(&mcrt, v1.EventTypeWarning, reasonTooManyCertificates, err.Error())
}
