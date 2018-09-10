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
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "managed-certs-gke/pkg/apis/gke.googleapis.com/v1alpha1"
	"managed-certs-gke/pkg/client"
	"managed-certs-gke/third_party/client/clientset/versioned"
	mcrtlister "managed-certs-gke/third_party/client/listers/gke.googleapis.com/v1alpha1"
)

type McrtController struct {
	mcrt   *versioned.Clientset
	lister mcrtlister.ManagedCertificateLister
	synced cache.InformerSynced
	queue  workqueue.RateLimitingInterface
	ssl    *client.Ssl
	state  *McrtState
}

func (c *McrtController) Run(stopChannel <-chan struct{}, errors chan<- error) {
	defer c.queue.ShutDown()

	err := c.initializeState()
	if err != nil {
		errors <- fmt.Errorf("Could not intialize state: %v", err)
		return
	}

	go wait.Until(c.runWorker, time.Second, stopChannel)
	go wait.Until(c.synchronizeAllMcrts, time.Minute, stopChannel)

	<-stopChannel
}

func (c *McrtController) initializeState() error {
	mcrts, err := c.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	var mcrtsNames []string
	for _, mcrt := range mcrts {
		mcrtsNames = append(mcrtsNames, mcrt.Name)
	}
	glog.Infof("McrtController initializing state, managed certificates found: %+v", strings.Join(mcrtsNames, ", "))

	for _, mcrt := range mcrts {
		c.state.Put(mcrt.Name, mcrt.Status.CertificateName)
	}

	return nil
}

func (c *McrtController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.queue.AddRateLimited(key)
	}
}

func (c *McrtController) getAllMcrtsInCluster() (result map[string]*api.ManagedCertificate, err error) {
	mcrts, err := c.lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	result = make(map[string]*api.ManagedCertificate, len(mcrts))
	for _, mcrt := range mcrts {
		result[mcrt.Name] = mcrt
	}

	return
}

func (c *McrtController) deleteObsoleteMcrtsFromState(allMcrtsInCluster map[string]*api.ManagedCertificate) {
	allKnownMcrts := c.state.GetAllManagedCertificates()

	glog.Infof("McrtController deleting obsolete managed certificates from state, all known in state: %+v", allKnownMcrts)

	for _, knownMcrt := range allKnownMcrts {
		if _, exists := allMcrtsInCluster[knownMcrt]; !exists {
			// A managed certificate exists in state, but does not exist as a custom object in cluster, probably was deleted by the user - delete it from the state.
			c.state.Delete(knownMcrt)
			glog.Infof("McrtController deleted managed certificate %s from state - not found in cluster", knownMcrt)
		}
	}
}

func (c *McrtController) deleteObsoleteSslCertificates() error {
	allKnownSslCerts := c.state.GetAllSslCertificates()
	glog.Infof("McrtController deleting obsolete SslCertificates from project, all known in state: %+v", allKnownSslCerts)

	allKnownSslCertsSet := make(map[string]bool, len(allKnownSslCerts))

	for _, knownSslCert := range allKnownSslCerts {
		allKnownSslCertsSet[knownSslCert] = true
	}

	sslCerts, err := c.ssl.List()
	if err != nil {
		return err
	}

	var sslCertsString []string
	for _, sslCert := range sslCerts.Items {
		sslCertsString = append(sslCertsString, sslCert.Name)
	}

	glog.Infof("McrtController deleting obsolete SslCertificates from project, all existing in project: %+v", sslCertsString)

	for _, sslCert := range sslCerts.Items {
		if known, exists := allKnownSslCertsSet[sslCert.Name]; !exists || !known {
			c.ssl.Delete(sslCert.Name)
			glog.Infof("McrtController deleted SslCertificate %s from project - not found in state", sslCert.Name)
		}
	}

	return nil
}

func (c *McrtController) synchronizeAllMcrts() {
	allMcrtsInCluster, err := c.getAllMcrtsInCluster()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	var mcrts []string
	for mcrtName := range allMcrtsInCluster {
		mcrts = append(mcrts, mcrtName)
	}
	glog.Infof("McrtController synchronizing managed certificates, all found in cluster: %+v", mcrts)

	c.deleteObsoleteMcrtsFromState(allMcrtsInCluster)

	err = c.deleteObsoleteSslCertificates()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, mcrt := range allMcrtsInCluster {
		c.enqueue(mcrt)
	}
}
