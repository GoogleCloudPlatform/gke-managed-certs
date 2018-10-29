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

package client

import (
	"fmt"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func (c *Clients) DeleteAllManagedCertificates(namespace string) error {
	nsClient := c.Clientset.GkeV1alpha1().ManagedCertificates(namespace)
	mcrts, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mcrt := range mcrts.Items {
		if err := http.IgnoreNotFound(nsClient.Delete(mcrt.Name, &metav1.DeleteOptions{})); err != nil {
			return err
		}
	}

	return nil
}

func (c *Clients) DeleteAllSslCertificates() error {
	names, err := c.getSslCertificatesNames()
	if err != nil {
		return err
	}

	glog.Infof("Delete SslCertificates: %v", names)

	if err := c.deleteSslCertificates(names); err != nil {
		return err
	}

	names, err = c.getSslCertificatesNames()
	if err != nil {
		return err
	}

	if len(names) > 0 {
		return fmt.Errorf("SslCertificates not deleted: %v", names)
	}

	return nil
}

func (c *Clients) deleteSslCertificates(names []string) error {
	for _, name := range names {
		_, err := c.Compute.SslCertificates.Delete(c.ProjectID, name).Do()
		if !http.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (c *Clients) getSslCertificatesNames() ([]string, error) {
	sslCertificates, err := c.Compute.SslCertificates.List(c.ProjectID).Do()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, sslCertificate := range sslCertificates.Items {
		names = append(names, sslCertificate.Name)
	}

	return names, nil
}
