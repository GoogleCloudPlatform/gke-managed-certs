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

package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"managed-certs-gke/pkg/ingress"
	"managed-certs-gke/pkg/sslcertificate"
	"managed-certs-gke/third_party/client/clientset/versioned"

	mcertlister "managed-certs-gke/third_party/client/listers/alpha.cloud.google.com/v1alpha1"
)

type IngressController struct {
	client *ingress.Interface
	queue  workqueue.RateLimitingInterface
}

type McertController struct {
	client    *versioned.Clientset
	lister    mcertlister.ManagedCertificateLister
	synced    cache.InformerSynced
	queue     workqueue.RateLimitingInterface
	sslClient *sslcertificate.SslClient
	state     *McertState
}

type Controller struct {
	Ingress IngressController
	Mcert   McertController
}

// [review]: not good style to have a separate interface.go file...
