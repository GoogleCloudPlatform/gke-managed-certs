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
	"sort"
	"strings"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"

	"managed-certs-gke/pkg/utils"
)

func (c *Controller) updatePreSharedCertAnnotation() {
	ingresses, err := c.Ingress.client.List()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	sslCerts, err := c.Mcert.sslClient.List()
	if err != nil {
		runtime.HandleError(err)
		return
	}
	sslCertsMap := make(map[string]bool, 0)
	for _, sslCert := range sslCerts.Items {
		sslCertsMap[sslCert.Name] = true
	}

	for _, ingress := range ingresses.Items {
		glog.Infof("Update pre-shared-cert annotation for ingress %s", ingress.Name)

		mcerts, exists := utils.ParseAnnotation(&ingress)
		if !exists {
			continue
		}

		var sslCertNames []string
		for _, mcert := range mcerts {
			if sslCertName, exists := c.Mcert.state.Get(mcert); exists {
				if _, exists := sslCertsMap[sslCertName]; exists {
					sslCertNames = append(sslCertNames, sslCertName)
				}
			}
		}

		if len(sslCertNames) < 1 {
			glog.Infof("No ssl certificates to update ingress %s with", ingress.Name)
			continue
		}

		sort.Strings(sslCertNames)

		glog.Infof("Update pre-shared-cert annotation for ingress %s, found SslCertificate resource names to put in the annotation: %s", ingress.Name, strings.Join(sslCertNames, ", "))

		ingress.Annotations["ingress.gcp.kubernetes.io/pre-shared-cert"] = strings.Join(sslCertNames, ", ")
		if _, err := c.Ingress.client.Update(&ingress); err != nil {
			runtime.HandleError(err)
			return
		}

		glog.Infof("Annotation pre-shared-cert updated for ingress %s", ingress.Name)
	}

}
