package managedcertificate

import (
	"fmt"
	"k8s.io/client-go/rest"
	"managed-certs-gke/pkg/client/clientset/versioned"
	"managed-certs-gke/pkg/client/informers/externalversions"
)

func Init() (*versioned.Clientset, *externalversions.SharedInformerFactory, error) {
        config, err := rest.InClusterConfig()
        if err != nil {
                return nil, nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
        }
	client := versioned.NewForConfigOrDie(config)
	factory := externalversions.NewSharedInformerFactory(client, 0)
	return client, &factory, nil
}
