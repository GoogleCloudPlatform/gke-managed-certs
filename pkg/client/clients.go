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
package client

import (
	"fmt"

	"k8s.io/client-go/rest"

	"managed-certs-gke/third_party/client/clientset/versioned"
	"managed-certs-gke/third_party/client/informers/externalversions"
)

type Clients struct {
	// ConfigMap manages ConfigMap objects
	ConfigMap ConfigMapClient

	// Ingress manages Ingress objects
	Ingress *Ingress

	// Mcrt manages ManagedCertificate custom resources
	Mcrt *versioned.Clientset

	// McrtInfomerFactory produces informers and listers which handle ManagedCertificate custom resources
	McrtInformerFactory externalversions.SharedInformerFactory

	// Ssl manages SslCertificate GCP resource
	Ssl *Ssl
}

func NewClients(cloudConfig string) (*Clients, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
	}

	mcrt := versioned.NewForConfigOrDie(config)
	factory := externalversions.NewSharedInformerFactory(mcrt, 0)

	ssl, err := NewSsl(cloudConfig)
	if err != nil {
		return nil, err
	}

	return &Clients{
		ConfigMap:           NewConfigMap(config),
		Ingress:             NewIngress(config),
		Mcrt:                mcrt,
		McrtInformerFactory: factory,
		Ssl:                 ssl,
	}, nil
}
