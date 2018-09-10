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
* Provides operations for manipulating Ingress objects. Offers List() and Watch() operations which handle Ingress objects in all namespaces.
 */
package client

import (
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

type Ingress struct {
	client *v1beta1.ExtensionsV1beta1Client
}

func NewIngress(config *rest.Config) *Ingress {
	return &Ingress{
		client: v1beta1.NewForConfigOrDie(config),
	}
}

/*
* Fetches a given Ingress object.
 */
func (c *Ingress) Get(namespace string, name string) (*api.Ingress, error) {
	return c.client.Ingresses(namespace).Get(name, v1.GetOptions{})
}

/*
* Lists all Ingress objects in the cluster, from all namespaces.
 */
func (c *Ingress) List() (*api.IngressList, error) {
	var result api.IngressList
	err := c.client.RESTClient().Get().Resource(resource).Do().Into(&result)
	return &result, err
}

/*
* Updates a given Ingress object.
 */
func (c *Ingress) Update(ingress *api.Ingress) (*api.Ingress, error) {
	return c.client.Ingresses(ingress.Namespace).Update(ingress)
}

/*
* Watches all Ingress objects in the cluster, from all namespaces.
 */
func (c *Ingress) Watch() (watch.Interface, error) {
	opts := &v1.ListOptions{Watch: true}
	return c.client.RESTClient().Get().Resource(resource).VersionedParams(opts, scheme.ParameterCodec).Watch()
}
