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

package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
)

const (
	maxRetries = 60
	timeout    = 30
)

var retryError = errors.New("Retry failed")

func Retry(action func() error) error {
	for i := 1; i <= maxRetries; i++ {
		err := action()
		if err == nil {
			return nil
		}

		if i < maxRetries {
			glog.Warningf("%d. retry in %d seconds because of %s", i, timeout, err.Error())
			time.Sleep(timeout * time.Second)
		}
	}

	glog.Errorf("Failed because of exceeding the retry limit")
	return retryError
}

func Setup(t *testing.T, namespace string) *client.Clients {
	client, err := client.New()
	if err != nil {
		t.Fatalf("Could not create client: %s", err.Error())
	}

	TearDown(t, client, namespace)
	return client
}

func TearDown(t *testing.T, client *client.Clients, namespace string) {
	err := func() error {
		if err := client.ManagedCertificate.DeleteAll(namespace); err != nil {
			return err
		}

		if err := Retry(client.SslCertificate.DeleteOwn); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		t.Errorf("Failed to tear down resources: %s", err.Error())
	}
}
