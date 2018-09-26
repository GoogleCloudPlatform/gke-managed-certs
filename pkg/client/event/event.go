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

// Package event provides operations for manipulating Event objects.
package event

import (
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
)

const (
	component             = "managed-certificate-controller"
	namespace             = ""
	tooManyCertificates   = "TooManyCertificates"
	transientBackendError = "TransientBackendError"
)

type Event struct {
	recorder record.EventRecorder
}

// New creates an event recorder to send custom events to Kubernetes.
func New(client kubernetes.Interface) *Event {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.V(4).Infof)
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1.New(client.CoreV1().RESTClient()).Events(namespace)})
	return &Event{
		recorder: broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: component}),
	}
}

// TooManyCertificates creates an event if quota for maximum number of SslCertificates per GCP project is exceeded.
func (c *Event) TooManyCertificates(mcrt *api.ManagedCertificate, err error) {
	c.recorder.Event(mcrt, v1.EventTypeWarning, tooManyCertificates, err.Error())
}

// TransientBackendError creates an event if a transient error occurrs when calling GCP API.
func (c *Event) TransientBackendError(mcrt *api.ManagedCertificate, err error) {
	c.recorder.Event(mcrt, v1.EventTypeWarning, transientBackendError, err.Error())
}
