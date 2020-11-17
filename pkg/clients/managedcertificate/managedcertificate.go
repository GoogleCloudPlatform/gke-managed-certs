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

// Package managedcertificate exposes the interface the controller needs
// to operate on ManagedCertificate resources.
package managedcertificate

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	clientsetv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions"
	informersv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions/networking.gke.io/v1"
	queueutils "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/queue"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Interface defines the interface the controller needs to operate
// on ManagedCertificate resources.
type Interface interface {
	// Get fetches the resource identified by id.
	Get(id types.Id) (*apisv1.ManagedCertificate, error)
	// HasSynced is true after first batch of ManagedCertificate
	// resources defined in the cluster has been synchronized with
	// the local storage.
	HasSynced() bool
	// List returns all ManagedCertificate resources.
	List() ([]*apisv1.ManagedCertificate, error)
	// Update updates the given ManagedCertificate resource.
	Update(ctx context.Context, managedCertificate *apisv1.ManagedCertificate) error
	// Run initializes the object exposing the ManagedCertificate
	// API.
	Run(ctx context.Context, queue workqueue.RateLimitingInterface)
}

type impl struct {
	client   clientsetv1.NetworkingV1Interface
	factory  externalversions.SharedInformerFactory
	informer informersv1.ManagedCertificateInformer
}

func New(clientset *versioned.Clientset) Interface {
	factory := externalversions.NewSharedInformerFactory(clientset, 0)

	return impl{
		client:   clientset.NetworkingV1(),
		factory:  factory,
		informer: factory.Networking().V1().ManagedCertificates(),
	}
}

func (m impl) Get(id types.Id) (*apisv1.ManagedCertificate, error) {
	return m.informer.Lister().ManagedCertificates(id.Namespace).Get(id.Name)
}

func (m impl) HasSynced() bool {
	return m.informer.Informer().HasSynced()
}

func (m impl) List() ([]*apisv1.ManagedCertificate, error) {
	return m.informer.Lister().List(labels.Everything())
}

func (m impl) Update(ctx context.Context, managedCertificate *apisv1.ManagedCertificate) error {
	_, err := m.client.ManagedCertificates(managedCertificate.Namespace).
		Update(ctx, managedCertificate, metav1.UpdateOptions{})
	return err
}

func (m impl) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {
	go m.factory.Start(ctx.Done())

	m.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { queueutils.Add(queue, obj) },
		UpdateFunc: func(old, new interface{}) { queueutils.Add(queue, new) },
		DeleteFunc: func(obj interface{}) { queueutils.Add(queue, obj) },
	})
}
