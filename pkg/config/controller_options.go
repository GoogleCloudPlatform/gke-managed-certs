package config

import (
	"fmt"
	"k8s.io/client-go/rest"
	"managed-certs-gke/pkg/client/clientset/versioned"
	"managed-certs-gke/pkg/client/informers/externalversions"
	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
)

type ControllerOptions struct {
	// IngressClient is a rest client which operates on k8s Ingress objects
	IngressClient *ingress.Interface

	// McertClient lets manage ManagedCertificate custom resources
	McertClient *versioned.Clientset

	// McertInfomerFactory produces informers and listers which handle ManagedCertificate custom resources
	McertInformerFactory externalversions.SharedInformerFactory

	// SslClient is a client of GCP compute api which gives access to SslCertificate resource
	SslClient *sslcertificate.SslClient
}

func NewControllerOptions(cloudConfig string) (*ControllerOptions, error) {
	ingressClient, err := ingress.NewClient()
	if err != nil {
		return nil, err
	}

        config, err := rest.InClusterConfig()
        if err != nil {
                return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
        }
	client := versioned.NewForConfigOrDie(config)
	factory := externalversions.NewSharedInformerFactory(client, 0)

	sslClient, err := sslcertificate.NewClient(cloudConfig)
	if err != nil {
		return nil, err
	}

	return &ControllerOptions{
		IngressClient: ingressClient,
		McertClient: client,
		McertInformerFactory: factory,
		SslClient: sslClient,
	}, nil
}
