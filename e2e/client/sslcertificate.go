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
	"strings"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func (s sslCertificate) DeleteOwn() error {
	names, err := s.getOwnNames()
	if err != nil {
		return err
	}

	if len(names) == 0 {
		return nil
	}

	glog.Infof("Delete SslCertificates: %v", names)

	for _, name := range names {
		if err := s.Delete(name); err != nil {
			return err
		}
	}

	names, err = s.getOwnNames()
	if err != nil {
		return err
	}

	if len(names) > 0 {
		return fmt.Errorf("SslCertificates not deleted: %v", names)
	}

	return nil
}

func (s sslCertificate) Delete(name string) error {
	_, err := s.sslCertificates.Delete(s.projectID, name).Do()
	if http.IsNotFound(err) {
		return nil
	}

	return err
}

func (s sslCertificate) Get(name string) (*compute.SslCertificate, error) {
	return s.sslCertificates.Get(s.projectID, name).Do()
}

func (s sslCertificate) getOwnNames() ([]string, error) {
	sslCertificates, err := s.sslCertificates.List(s.projectID).Do()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, sslCertificate := range sslCertificates.Items {
		if strings.HasPrefix(sslCertificate.Name, config.SslCertificateNamePrefix) {
			names = append(names, sslCertificate.Name)
		}
	}

	return names, nil
}
