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
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	api "managed-certs-gke/pkg/apis/cloud.google.com/v1alpha1"
	"time"
)

func (c *McertController) Run(stopChannel <-chan struct{}, errors chan<- error) {
	defer c.queue.ShutDown()

	err := c.initializeState()
	if err != nil {
		errors <- fmt.Errorf("Could not intialize state: %v", err)
		return
	}

	go wait.Until(c.runWorker, time.Second, stopChannel)
	go wait.Until(c.synchronizeAllMcerts, time.Minute, stopChannel)

	<-stopChannel
}

func (c *McertController) initializeState() error {
	mcerts, err := c.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	glog.Infof("Initializing state, managed certificates found: %+v", mcerts)

	for _, mcert := range mcerts {
		glog.Infof("Initializing state, map managed certificate %s to ssl certificate %s", mcert.ObjectMeta.Name, mcert.Status.CertificateName)
		c.state.Put(mcert.ObjectMeta.Name, mcert.Status.CertificateName)
	}

	return nil
}

func (c *McertController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.queue.AddRateLimited(key)
	}
}

func (c *McertController) getAllMcertsInCluster() (result map[string]*api.ManagedCertificate, err error) {
	mcerts, err := c.lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	result = make(map[string]*api.ManagedCertificate, len(mcerts))
	for _, mcert := range mcerts {
		result[mcert.ObjectMeta.Name] = mcert
	}

	return
}

func (c *McertController) deleteObsoleteMcertsFromState(allMcertsInCluster map[string]*api.ManagedCertificate) {
	allKnownMcerts := c.state.GetAllManagedCertificates()

	glog.Infof("Deleting obsolete managed certificates, managed certificates in state: %+v", allKnownMcerts)

	for _, knownMcert := range allKnownMcerts {
		if _, exists := allMcertsInCluster[knownMcert]; !exists {
			// A managed certificate exists in state, but does not exist as a custom object in cluster, probably was deleted by the user - delete it from the state.
			c.state.Delete(knownMcert)
			glog.Infof("Deleted %s managed certificate from state, because such custom object does not exist in the cluster (any more?)", knownMcert)
		}
	}
}

func (c* McertController) deleteObsoleteSslCertificates() error {
	allKnownSslCerts := c.state.GetAllSslCertificates()
	glog.Infof("Deleting obsolete SslCertificates, all SslCertificates in state: %+v", allKnownSslCerts)

	allKnownSslCertsSet := make(map[string]bool, len(allKnownSslCerts))

	for _, knownSslCert := range allKnownSslCerts {
		allKnownSslCertsSet[knownSslCert] = true
	}

	sslCerts, err := c.sslClient.List()
	if err != nil {
		return err
	}

	glog.Infof("Deleting obsolete SslCertificates, all SslCertificates in project: %+v", sslCerts)

	for _, sslCert := range sslCerts.Items {
		if known, exists := allKnownSslCertsSet[sslCert.Name]; !exists || !known {
			c.sslClient.Delete(sslCert.Name)
			glog.Infof("Deleted %s SslCertificate resource, because there is no such ssl certificate in state", sslCert.Name)
		}
	}

	return nil
}

func (c *McertController) synchronizeAllMcerts() {
	allMcertsInCluster, err := c.getAllMcertsInCluster()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	glog.Infof("Synchronizing managed certificates, all managed certificates found in cluster: %+v", allMcertsInCluster)

	c.deleteObsoleteMcertsFromState(allMcertsInCluster)

	err = c.deleteObsoleteSslCertificates()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, mcert := range allMcertsInCluster {
		c.enqueue(mcert)
	}
}
