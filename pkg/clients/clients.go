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

// Package clients provides clients which are used to communicate with api server and GCLB.
package clients

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
)

// Clients are used to communicate with api server and GCLB
type Clients struct {
	// Clientset manages ManagedCertificate custom resources
	Clientset versioned.Interface

	// ConfigMap manages ConfigMap objects
	ConfigMap configmap.ConfigMap

	// Event manages Event objects
	Event event.Event

	// InfomerFactory produces informers and listers which handle ManagedCertificate custom resources
	InformerFactory externalversions.SharedInformerFactory

	// Ssl manages SslCertificate GCP resources
	Ssl ssl.Ssl
}

func New(config *config.Config) (*Clients, error) {
	clusterConfig, err := clientcmd.BuildConfigFromFlags(flags.F.APIServerHost, flags.F.KubeConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
	}

	clientset := versioned.NewForConfigOrDie(clusterConfig)
	factory := externalversions.NewSharedInformerFactory(clientset, 0)

	ssl, err := ssl.New(config)
	if err != nil {
		return nil, err
	}

	event, err := event.New(kubernetes.NewForConfigOrDie(clusterConfig))
	if err != nil {
		return nil, err
	}

	return &Clients{
		Clientset:       clientset,
		ConfigMap:       configmap.New(clusterConfig),
		Event:           event,
		InformerFactory: factory,
		Ssl:             ssl,
	}, nil
}
