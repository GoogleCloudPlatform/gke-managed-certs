package main

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	mcertclient "managed-certs-gke/pkg/client/clientset/versioned"
	mcertinformer "managed-certs-gke/pkg/client/informers/externalversions"
	mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
	"managed-certs-gke/pkg/ingress"
	//mcert "managed-certs-gke/pkg/managedcertificate"
)

type Controller struct {
	ingressClient rest.Interface
	mcertLister mcertlister.ManagedCertificateLister
	mcertSynced cache.InformerSynced
}

func NewController(ingressClient rest.Interface, mcertClient *mcertclient.Clientset, mcertInformerFactory mcertinformer.SharedInformerFactory) *Controller {
	mcertInformer := mcertInformerFactory.Cloud().V1alpha1().ManagedCertificates()

	controller := &Controller{
		ingressClient: ingressClient,
		mcertLister: mcertInformer.Lister(),
		mcertSynced: mcertInformer.Informer().HasSynced,
	}

	mcertInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
		},
		UpdateFunc: func(old, new interface{}) {
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	return controller
}

func (c *Controller) Run(stopChannel <-chan struct{}) error {
	defer runtime.HandleCrash()

	glog.Info("Controller.Run()")

	glog.Info("Waiting for managedcertificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.mcertSynced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	glog.Info("Cache synced")

	ingresses, err := ingress.List(c.ingressClient)
	if err != nil {
		runtime.HandleError(err)
		return err
	}

	for _, x := range ingresses.Items {
		glog.Infof("%v", x.ObjectMeta.Name)
	}

	glog.Info("Waiting for stop signal")
	<-stopChannel
	glog.Info("Received stop signal")

	glog.Info("Shutting down")
	return nil
}
