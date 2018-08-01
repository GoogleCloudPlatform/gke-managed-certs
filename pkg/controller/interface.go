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

package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
	"managed-certs-gke/third_party/client/clientset/versioned"
	mcertlister "managed-certs-gke/third_party/client/listers/cloud.google.com/v1alpha1"
	"sync"
)

type IngressController struct {
        client *ingress.Interface
        queue workqueue.RateLimitingInterface
}

type McertState struct {
	sync.RWMutex

	// Maps ManagedCertificate name to SslCertificate name
	m map[string]string
}

func newMcertState() *McertState {
	return &McertState{
		m: make(map[string]string),
	}
}

func (state *McertState) Delete(key string) {
	state.Lock()
	defer state.Unlock()
	delete(state.m, key)
}

func (state *McertState) Get(key string) (value string, exists bool) {
	state.RLock()
	defer state.RUnlock()
	value, exists = state.m[key]

	return
}

func (state *McertState) GetAllManagedCertificates() (values []string) {
	values = make([]string, 0)

	state.RLock()
	defer state.RUnlock()

	for key := range state.m {
		values = append(values, key)
	}

	return
}

func (state *McertState) GetAllSslCertificates() (values []string) {
	values = make([]string, 0)

	state.RLock()
	defer state.RUnlock()

	for _, value := range state.m {
		values = append(values, value)
	}

	return
}

func (state *McertState) Put(key, value string) {
	state.Lock()
	defer state.Unlock()
	state.m[key] = value
}

type McertController struct {
	client *versioned.Clientset
        lister mcertlister.ManagedCertificateLister
        synced cache.InformerSynced
        queue workqueue.RateLimitingInterface
	sslClient *sslcertificate.SslClient
	state *McertState
}

type Controller struct {
	Ingress IngressController
	Mcert McertController
}
