// Handles all Ingress objects in the cluster, from all namespaces.
package ingress

import (
	"fmt"
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

const (
	resource = "ingresses"
)

func NewClient() (client rest.Interface, err error) {
        config, err := rest.InClusterConfig()
        if err != nil {
                return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
        }

        return v1beta1.NewForConfigOrDie(config).RESTClient(), nil
}

func List(client rest.Interface) (result *api.IngressList, err error) {
	result = &api.IngressList{}
	err = client.Get().Resource(resource).Do().Into(result)
	return
}
