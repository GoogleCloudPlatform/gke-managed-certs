package controller

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	mcertclient "managed-certs-gke/pkg/client/clientset/versioned"
	mcertinformer "managed-certs-gke/pkg/client/informers/externalversions"
	//mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
	"managed-certs-gke/pkg/ingress"
	//mcert "managed-certs-gke/pkg/managedcertificate"
	"time"
)

func (c *Controller) enqueueIngress(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.ingressQueue.AddRateLimited(key)
	}
}

func (c *Controller) enqueueMcert(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.mcertQueue.AddRateLimited(key)
	}
}

func NewController(ingressClient rest.Interface, mcertClient *mcertclient.Clientset, mcertInformerFactory mcertinformer.SharedInformerFactory) *Controller {
	mcertInformer := mcertInformerFactory.Cloud().V1alpha1().ManagedCertificates()

	controller := &Controller{
		ingressClient: ingressClient,
		ingressQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		mcertLister: mcertInformer.Lister(),
		mcertSynced: mcertInformer.Informer().HasSynced,
		mcertQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mcertQueue"),
	}

	mcertInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueueMcert(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueMcert(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueMcert(obj)
		},
	})

	return controller
}

func (c *Controller) Run(stopChannel <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.ingressQueue.ShutDown()
	defer c.mcertQueue.ShutDown()

	glog.Info("Controller.Run()")

	glog.Info("Waiting for managedcertificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.mcertSynced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	glog.Info("Cache synced")

	go c.runIngressWatcher()
	go wait.Until(c.runIngressWorker, time.Second, stopChannel)
	go wait.Until(c.runMcertWorker, time.Second, stopChannel)

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
