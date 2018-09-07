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

/*
* Provides clients which managed-certificate-controller uses to talk to api server and GCLB, to perform its tasks.
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

type ControllerClients struct {
	// Ingress is a client which manages Ingress objects
	Ingress *ingress.Interface

	// Mcert is a client which manages ManagedCertificate custom resources
	Mcert *versioned.Clientset

	// McertInfomerFactory produces informers and listers which handle ManagedCertificate custom resources
	McertInformerFactory externalversions.SharedInformerFactory

	// SslClient is a client of GCP compute api which gives access to SslCertificate resource
	Ssl *sslcertificate.SslClient
}

func NewControllerClients(cloudConfig string) (*ControllerClients, error) {
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

	return &ControllerClients{
		Ingress:              ingressClient,
		Mcert:                client,
		McertInformerFactory: factory,
		Ssl:                  sslClient,
	}, nil
}
