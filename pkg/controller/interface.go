package controller

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
)

type IngressController struct {
        client rest.Interface
        queue workqueue.RateLimitingInterface
}

type Controller struct {
	Ingress IngressController
        mcertLister mcertlister.ManagedCertificateLister
        mcertSynced cache.InformerSynced
        mcertQueue workqueue.RateLimitingInterface
}
