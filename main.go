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

package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/server"

	"managed-certs-gke/pkg/config"
	"managed-certs-gke/pkg/controller"
)

const managedCertificatesVersion = "0.0.1"

var cloudConfig = flag.String("cloud-config", "", "The path to the cloud provider configuration file.  Empty string for no configuration file.")
var ingressWatcherDelay = flag.Duration("ingress-watcher-delay", time.Second, "The delay slept before polling for Ingress resources")

func main() {
	glog.V(1).Infof("Managed certificates %s controller starting", managedCertificatesVersion)

	//To handle SIGINT gracefully
	stopChannel := server.SetupSignalHandler()

	clients, err := config.NewControllerClients(*cloudConfig)
	if err != nil {
		glog.Fatal(err)
	}

	controller := controller.NewController(clients)

	go clients.McertInformerFactory.Start(stopChannel)

	if err = controller.Run(stopChannel, *ingressWatcherDelay); err != nil {
		glog.Fatal("Error running controller: %v", err)
	}
}
