// Handles all Ingress objects in the cluster, from all namespaces.
package ingress

import (
	"fmt"
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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

func Get(client rest.Interface, namespace string, name string) (result *api.Ingress, err error) {
	result = &api.Ingress{}
	err = client.Get().Namespace(namespace).Resource(resource).Name(name).Do().Into(result)
	return
}

func List(client rest.Interface) (result *api.IngressList, err error) {
	result = &api.IngressList{}
	err = client.Get().Resource(resource).Do().Into(result)
	return
}

func Watch(client rest.Interface) (watch.Interface, error) {
	opts := &v1.ListOptions{Watch: true}
	return client.Get().Resource(resource).VersionedParams(opts, scheme.ParameterCodec).Watch()
}
