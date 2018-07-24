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
	"time"
)

func NewController(ingressClient rest.Interface, mcertClient *mcertclient.Clientset, mcertInformerFactory mcertinformer.SharedInformerFactory) *Controller {
	mcertInformer := mcertInformerFactory.Cloud().V1alpha1().ManagedCertificates()

	controller := &Controller{
		Ingress: IngressController{
			client: ingressClient,
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		},
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
	defer c.Ingress.queue.ShutDown()
	defer c.mcertQueue.ShutDown()

	glog.Info("Controller.Run()")

	glog.Info("Waiting for managedcertificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.mcertSynced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	glog.Info("Cache synced")

	go c.Ingress.runWatcher()
	go wait.Until(c.runIngressWorker, time.Second, stopChannel)
	go wait.Until(c.runMcertWorker, time.Second, stopChannel)
	go wait.Until(c.Ingress.enqueueAll, 15*time.Minute, stopChannel)
	go wait.Until(c.enqueueAllMcerts, 15*time.Minute, stopChannel)

	glog.Info("Waiting for stop signal")
	<-stopChannel
	glog.Info("Received stop signal")

	glog.Info("Shutting down")
	return nil
}
