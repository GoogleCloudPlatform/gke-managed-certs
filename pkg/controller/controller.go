package controller

import (
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
			state: newMcertState(),
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

	glog.Info("Controller.Run()")

	errors := make(chan error)

	mcertStopChannel := make(chan struct{})
	go c.Mcert.Run(mcertStopChannel, errors)

	ingressStopChannel := make(chan struct{})
	go c.Ingress.Run(ingressStopChannel)

	go wait.Until(c.runIngressWorker, time.Second, stopChannel)

	glog.Info("Waiting for stop signal or error")
	select{
		case <-stopChannel:
			glog.Info("Received stop signal")
			quit(mcertStopChannel, ingressStopChannel)
		case err := <-errors:
			runtime.HandleError(err)
			quit(mcertStopChannel, ingressStopChannel)
	}

	glog.Info("Shutting down")
	return nil
}

func quit(mcertStopChannel, ingressStopChannel chan<- struct{}) {
	mcertStopChannel <- struct{}{}
	ingressStopChannel <- struct{}{}
}
