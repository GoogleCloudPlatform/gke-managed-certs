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
		// [review] you need to glog errorf here
		runtime.HandleError(err) // [review] is this what we want to do to handle errors?
		return
	}

	sslCerts, err := c.Mcert.sslClient.List()
	if err != nil {
		// [review] you need to glog errorf here, see comment above
		runtime.HandleError(err)
		return
	}
	sslCertsMap := map[string]bool{}
	for _, sslCert := range sslCerts.Items {
		sslCertsMap[sslCert.Name] = true
	}

	for _, ingress := range ingresses.Items {
		glog.Infof("Update pre-shared-cert annotation for ingress %v", ingress.ObjectMeta.Name)

		mcerts, ok := utils.ParseAnnotation(&ingress)
		if !ok {
			// glog.errorf
			continue
		}

		var sslCertNames []string
		for _, mcert := range mcerts {
			if state, exists := c.Mcert.state.Get(mcert); exists {
				sslCertName := state.New
				if sslCertName == "" {
					sslCertName = state.Current
				}

				if _, exists := sslCertsMap[sslCertName]; exists {
					sslCertNames = append(sslCertNames, sslCertName)
				}
			}
		}

		if len(sslCertNames) < 1 {
			glog.Infof("No ssl certificates to update ingress %v with", ingress.ObjectMeta.Name)
			continue
		}

		sort.Strings(sslCertNames)

		glog.Infof("Update pre-shared-cert annotation for ingress %v, found SslCertificate resource names to put in the annotation: %v", ingress.ObjectMeta.Name, strings.Join(sslCertNames, ", "))

		// [review] vendor the annotation name from ingress-gce repo if possible
		ingress.ObjectMeta.Annotations["ingress.gcp.kubernetes.io/pre-shared-cert"] = strings.Join(sslCertNames, ", ")
		_, err := c.Ingress.client.Update(&ingress)
		if err != nil {
			runtime.HandleError(err)
			return
		}

		glog.Infof("Annotation pre-shared-cert updated for ingress %s:%s", ingress.Namespace, ingress.Name)
	}
}
