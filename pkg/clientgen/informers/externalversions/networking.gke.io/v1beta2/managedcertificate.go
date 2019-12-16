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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta2

import (
	time "time"

	networkinggkeiov1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	versioned "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	internalinterfaces "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions/internalinterfaces"
	v1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ManagedCertificateInformer provides access to a shared informer and lister for
// ManagedCertificates.
type ManagedCertificateInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta2.ManagedCertificateLister
}

type managedCertificateInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewManagedCertificateInformer constructs a new informer for ManagedCertificate type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewManagedCertificateInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredManagedCertificateInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredManagedCertificateInformer constructs a new informer for ManagedCertificate type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredManagedCertificateInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.NetworkingV1beta2().ManagedCertificates(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.NetworkingV1beta2().ManagedCertificates(namespace).Watch(options)
			},
		},
		&networkinggkeiov1beta2.ManagedCertificate{},
		resyncPeriod,
		indexers,
	)
}

func (f *managedCertificateInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredManagedCertificateInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *managedCertificateInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&networkinggkeiov1beta2.ManagedCertificate{}, f.defaultInformer)
}

func (f *managedCertificateInformer) Lister() v1beta2.ManagedCertificateLister {
	return v1beta2.NewManagedCertificateLister(f.Informer().GetIndexer())
}
