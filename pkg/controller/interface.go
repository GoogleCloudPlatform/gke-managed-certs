package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
)

type IngressController struct {
        client *ingress.Interface
        queue workqueue.RateLimitingInterface
}

type McertController struct {
        lister mcertlister.ManagedCertificateLister
        synced cache.InformerSynced
        queue workqueue.RateLimitingInterface
	sslClient *sslcertificate.SslClient
}

type Controller struct {
	Ingress IngressController
	Mcert McertController
}
