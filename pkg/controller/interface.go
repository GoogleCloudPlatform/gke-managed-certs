package controller

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
)

type Controller struct {
        ingressClient rest.Interface
        ingressQueue workqueue.RateLimitingInterface
        mcertLister mcertlister.ManagedCertificateLister
        mcertSynced cache.InformerSynced
        mcertQueue workqueue.RateLimitingInterface
}
