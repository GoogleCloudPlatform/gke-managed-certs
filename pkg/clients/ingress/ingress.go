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

// Package ingress exposes the interface the controller needs
// to operate on Ingress resources.
package ingress

import (
	"context"

	"k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	ingressutils "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/ingress"
	queueutils "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/queue"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Interface defines the interface the controller needs to operate
// on Ingress resources.
type Interface interface {
	// Get fetches the resource identified by id.
	Get(id types.Id) (*v1.Ingress, error)
	// HasSynced is true after first batch of Ingress resources
	// defined in the cluster has been synchronized with
	// the local storage.
	HasSynced() bool
	// List returns all Ingress resources.
	List() ([]*v1.Ingress, error)
	// Update updates the given Ingress resource.
	Update(ctx context.Context, ingress *v1.Ingress) error
	// Run initializes the object exposing the Ingress API.
	Run(ctx context.Context, queue workqueue.RateLimitingInterface)
}

type impl struct {
	client   typedv1.NetworkingV1Interface
	factory  informers.SharedInformerFactory
	informer informersv1.IngressInformer
}

func New(clientset *kubernetes.Clientset) Interface {
	factory := informers.NewSharedInformerFactory(clientset, 0)

	return impl{
		client:   clientset.NetworkingV1(),
		factory:  factory,
		informer: factory.Networking().V1().Ingresses(),
	}
}

func (ing impl) Get(id types.Id) (*v1.Ingress, error) {
	return ing.informer.Lister().Ingresses(id.Namespace).Get(id.Name)
}

func (ing impl) HasSynced() bool {
	return ing.informer.Informer().HasSynced()
}

func (ing impl) List() ([]*v1.Ingress, error) {
	return ing.informer.Lister().List(labels.Everything())
}

func (ing impl) Update(ctx context.Context, ingress *v1.Ingress) error {
	_, err := ing.client.Ingresses(ingress.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	return err
}

func (ing impl) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {
	go ing.factory.Start(ctx.Done())

	ing.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !ingressutils.IsGKE(obj) {
				klog.Info("Ignoring add for Ingress")
				return
			}

			queueutils.Add(queue, obj)
		},
		UpdateFunc: func(old, new interface{}) {
			if !ingressutils.IsGKE(new) {
				klog.Info("Ignoring update for Ingress")
				return
			}

			queueutils.Add(queue, new)
		},
		DeleteFunc: func(obj interface{}) {
			if !ingressutils.IsGKE(obj) {
				klog.Info("Ignoring delete for Ingress")
				return
			}

			queueutils.Add(queue, obj)
		},
	})
}
