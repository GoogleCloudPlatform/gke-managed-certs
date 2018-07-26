package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
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

type McertController struct {
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
