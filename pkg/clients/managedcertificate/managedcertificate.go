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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	clientsetv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions"
	informersv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Interface defines the interface the controller needs to operate
// on ManagedCertificate resources.
type Interface interface {
	// Get fetches the resource identified by id.
	Get(id types.CertId) (*apisv1.ManagedCertificate, error)
	// HasSynced is true after first batch of ManagedCertificate
	// resources defined in the cluster has been synchronized with
	// the local storage.
	HasSynced() bool
	// List returns all ManagedCertificate resources.
	List() ([]*apisv1.ManagedCertificate, error)
	// Update updates the given ManagedCertificate resource.
	Update(managedCertificate *apisv1.ManagedCertificate) error
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

func (m impl) Get(id types.CertId) (*apisv1.ManagedCertificate, error) {
	return m.informer.Lister().ManagedCertificates(id.Namespace).Get(id.Name)
}

func (m impl) HasSynced() bool {
	return m.informer.Informer().HasSynced()
}

func (m impl) List() ([]*apisv1.ManagedCertificate, error) {
	return m.informer.Lister().List(labels.Everything())
}

func (m impl) Update(managedCertificate *apisv1.ManagedCertificate) error {
	_, err := m.client.ManagedCertificates(managedCertificate.Namespace).Update(managedCertificate)
	return err
}

func (m impl) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {
	go m.factory.Start(ctx.Done())

	m.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { add(queue, obj) },
		UpdateFunc: func(old, new interface{}) { add(queue, new) },
		DeleteFunc: func(obj interface{}) { add(queue, obj) },
	})
}

func add(queue workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.Infof("Enqueuing ManagedCertificate: %+v", obj)
	queue.AddRateLimited(key)
}
