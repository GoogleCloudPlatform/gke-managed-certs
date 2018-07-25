package controller

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"managed-certs-gke/pkg/config"
	"time"
)

func NewController(opts *config.ControllerOptions) *Controller {
	mcertInformer := opts.McertInformerFactory.Cloud().V1alpha1().ManagedCertificates()

	controller := &Controller{
		Ingress: IngressController{
			client: opts.IngressClient,
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		},
		Mcert: McertController{
			lister: mcertInformer.Lister(),
			synced: mcertInformer.Informer().HasSynced,
			queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mcertQueue"),
			sslClient: opts.SslClient,
		},
	}

	mcertInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.Mcert.enqueue(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.Mcert.enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.Mcert.enqueue(obj)
		},
	})

	return controller
}

func (c *Controller) Run(stopChannel <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.Ingress.queue.ShutDown()
	defer c.Mcert.queue.ShutDown()

	glog.Info("Controller.Run()")

	glog.Info("Waiting for managedcertificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.Mcert.synced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	glog.Info("Cache synced")

	go c.Ingress.runWatcher()
	go wait.Until(c.runIngressWorker, time.Second, stopChannel)
	go wait.Until(c.Mcert.runWorker, time.Second, stopChannel)

	go wait.Until(c.Ingress.enqueueAll, 15*time.Minute, stopChannel)
	go wait.Until(c.Mcert.enqueueAll, 15*time.Minute, stopChannel)

	glog.Info("Waiting for stop signal")
	<-stopChannel
	glog.Info("Received stop signal")

	glog.Info("Shutting down")
	return nil
}
