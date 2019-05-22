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
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	networkingv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
)

// Clients are used to communicate with api server and GCLB
type Clients struct {
	// ConfigMap manages ConfigMap objects
	ConfigMap configmap.ConfigMap

	// Coordination is used for electing master
	Coordination coordinationv1.CoordinationV1Interface

	// Core manages core Kubernetes objects
	Core corev1.CoreV1Interface

	// Event manages Event objects
	Event event.Event

	// IngressClient manages Ingress objects
	IngressClient v1beta1.IngressesGetter

	// IngressInformerFactory produces informers and listers which handle Ingress objects
	IngressInformerFactory informers.SharedInformerFactory

	// ManagedCertificateClient manages ManagedCertificate custom resources
	ManagedCertificateClient networkingv1beta1.NetworkingV1beta1Interface

	// ManagedCertificateInfomerFactory produces informers and listers which handle ManagedCertificate custom resources
	ManagedCertificateInformerFactory externalversions.SharedInformerFactory

	// Ssl manages SslCertificate GCP resources
	Ssl ssl.Ssl
}

func New(config *config.Config) (*Clients, error) {
	clusterConfig, err := clientcmd.BuildConfigFromFlags(flags.F.APIServerHost, flags.F.KubeConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
	}

	ingressClient := v1beta1.NewForConfigOrDie(clusterConfig)

	kubernetesClient := kubernetes.NewForConfigOrDie(clusterConfig)
	ingressFactory := informers.NewSharedInformerFactory(kubernetesClient, 0)

	managedCertificateClient := versioned.NewForConfigOrDie(clusterConfig)
	managedCertificateFactory := externalversions.NewSharedInformerFactory(managedCertificateClient, 0)

	oauthClient := oauth2.NewClient(oauth2.NoContext, config.Compute.TokenSource)
	oauthClient.Timeout = config.Compute.Timeout
	ssl, err := ssl.New(oauthClient, config.Compute.ProjectID)
	if err != nil {
		return nil, err
	}

	event, err := event.New(kubernetesClient)
	if err != nil {
		return nil, err
	}

	return &Clients{
		ConfigMap:                         configmap.New(clusterConfig),
		Coordination:                      kubernetesClient.CoordinationV1(),
		Core:                              kubernetesClient.CoreV1(),
		Event:                             event,
		IngressClient:                     ingressClient,
		IngressInformerFactory:            ingressFactory,
		ManagedCertificateClient:          managedCertificateClient.NetworkingV1beta1(),
		ManagedCertificateInformerFactory: managedCertificateFactory,
		Ssl:                               ssl,
	}, nil
}

func (c *Clients) Run(ctx context.Context) {
	go c.IngressInformerFactory.Start(ctx.Done())
	go c.ManagedCertificateInformerFactory.Start(ctx.Done())
}
