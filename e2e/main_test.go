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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

const (
	namespace = "default"
)

var clients *client.Clients

func TestMain(m *testing.M) {
	flag.Parse()

	if err := setUp(); err != nil {
		glog.Fatal(err)
	}

	exitCode := m.Run()

	if err := tearDown(); err != nil {
		glog.Fatal(err)
	}

	os.Exit(exitCode)
}

func setUp() error {
	glog.Infof("setting up")

	var err error
	if clients, err = client.New(); err != nil {
		return fmt.Errorf("Could not create clients: %s", err.Error())
	}

	err = tearDown()
	glog.Infof("set up finished")
	return err
}

func tearDown() error {
	glog.Infof("tearing down")

	err := func() error {
		if err := clients.ManagedCertificate.DeleteAll(namespace); err != nil {
			return err
		}

		if err := utils.Retry(clients.SslCertificate.DeleteOwn); err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return fmt.Errorf("Error tearing down resources: %s", err.Error())
	}

	glog.Infof("tear down success")
	return nil
}
