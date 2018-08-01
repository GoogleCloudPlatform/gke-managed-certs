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

package config

import (
	"fmt"
	"k8s.io/client-go/rest"
	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
	"managed-certs-gke/third_party/client/clientset/versioned"
	"managed-certs-gke/third_party/client/informers/externalversions"
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
