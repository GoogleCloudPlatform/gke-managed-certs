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
	"context"
	"os"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/server"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/version"
)

func main() {
	flags.Register()

	glog.V(1).Infof("managed-certificates-controller %s starting. Latest commit hash: %s", version.Version, version.GitCommit)
	for i, a := range os.Args {
		glog.V(0).Infof("argv[%d]: %q", i, a)
	}
	glog.V(1).Infof("Flags = %+v", flags.F)

	config, err := config.New(flags.F.GCEConfigFilePath)
	if err != nil {
		glog.Fatal(err)
	}

	clients, err := clients.New(config)
	if err != nil {
		glog.Fatal(err)
	}

	controller := controller.New(config, clients)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-server.SetupSignalHandler()
		cancel()
	}()

	if err = controller.Run(ctx); err != nil {
		glog.Fatalf("Error running controller: %s", err)
	}
}
