package main

import (
	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/server"

	ingress "managed-certs-gke/pkg/ingress"
	mcert "managed-certs-gke/pkg/managedcertificate"
)

const ManagedCertificatesVersion = "0.0.1"

func main() {
	glog.V(1).Infof("Managed certificates %s controller starting", ManagedCertificatesVersion)

	//To handle SIGINT gracefully
	stopChannel := server.SetupSignalHandler()

	ingressClient, err := ingress.NewClient()
	if err != nil {
		glog.Fatal(err)
	}

	mcertClient, mcertInformerFactory, err := mcert.Init()
	if err != nil {
		glog.Fatal(err)
	}

	controller := NewController(ingressClient, mcertClient, *mcertInformerFactory)

	go (*mcertInformerFactory).Start(stopChannel)

	if err = controller.Run(stopChannel); err != nil {
		glog.Fatal("Error running controller: %v", err)
	}
}
