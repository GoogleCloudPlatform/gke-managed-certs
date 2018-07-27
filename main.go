package main

import (
	"flag"
	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/server"
	kube_flag "k8s.io/apiserver/pkg/util/flag"
	"managed-certs-gke/pkg/config"
	"managed-certs-gke/pkg/controller"
)

const managedCertificatesVersion = "0.0.1"

var cloudConfig = flag.String("cloud-config", "", "The path to the cloud provider configuration file.  Empty string for no configuration file.")

func main() {
	kube_flag.InitFlags()

	glog.V(1).Infof("Managed certificates %s controller starting", managedCertificatesVersion)

	//To handle SIGINT gracefully
	stopChannel := server.SetupSignalHandler()

	opts, err := config.NewControllerOptions(*cloudConfig)
	if err != nil {
		glog.Fatal(err)
	}

	controller := controller.NewController(opts)

	go opts.McertInformerFactory.Start(stopChannel)

	if err = controller.Run(stopChannel); err != nil {
		glog.Fatal("Error running controller: %v", err)
	}
}
