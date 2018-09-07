/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
* Wrapper over client-go for handling Ingress object. It is different from the wrapped client, as it offers List() and Watch() operations in all namespaces, with an easier to use interface.
 */
package ingress

import (
	"fmt"

	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

const (
	resource = "ingresses"
)

type Interface struct {
	client *v1beta1.ExtensionsV1beta1Client
}

func NewClient() (client *Interface, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
	}

	return &Interface{
		client: v1beta1.NewForConfigOrDie(config),
	}, nil
}

/*
* Fetches a given Ingress object.
 */
func (c *Interface) Get(namespace string, name string) (*api.Ingress, error) {
	return c.client.Ingresses(namespace).Get(name, v1.GetOptions{})
}

/*
* Lists all Ingress objects in the cluster, from all namespaces.
 */
func (c *Interface) List() (*api.IngressList, error) {
	var result api.IngressList
	err := c.client.RESTClient().Get().Resource(resource).Do().Into(&result)
	return &result, err
}

/*
* Updates a given Ingress object.
 */
func (c *Interface) Update(ingress *api.Ingress) (*api.Ingress, error) {
	return c.client.Ingresses(ingress.Namespace).Update(ingress)
}

/*
* Watches all Ingress objects in the cluster, from all namespaces.
 */
func (c *Interface) Watch() (watch.Interface, error) {
	opts := &v1.ListOptions{Watch: true}
	return c.client.RESTClient().Get().Resource(resource).VersionedParams(opts, scheme.ParameterCodec).Watch()
}
