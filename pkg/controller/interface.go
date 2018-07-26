package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"managed-certs-gke/pkg/client/clientset/versioned"
	mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
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

func (state *McertState) Get(key string) (value string, exists bool) {
	state.RLock()
	defer state.RUnlock()
	value, exists = state.m[key]
	return
}

func (state *McertState) Put(key string, value string) {
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
