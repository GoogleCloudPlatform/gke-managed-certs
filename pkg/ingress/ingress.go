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

type Interface struct {
	client rest.Interface
}

func NewClient() (client *Interface, err error) {
        config, err := rest.InClusterConfig()
        if err != nil {
                return nil, fmt.Errorf("Could not fetch cluster config, err: %v", err)
        }

        return &Interface{
		client: v1beta1.NewForConfigOrDie(config).RESTClient(),
	}, nil
}

func (c *Interface) Get(namespace string, name string) (result *api.Ingress, err error) {
	result = &api.Ingress{}
	err = c.client.Get().Namespace(namespace).Resource(resource).Name(name).Do().Into(result)
	return
}

func (c *Interface) List() (result *api.IngressList, err error) {
	result = &api.IngressList{}
	err = c.client.Get().Resource(resource).Do().Into(result)
	return
}

func (c *Interface) Update(ingress *api.Ingress) (result *api.Ingress, err error) {
	result = &api.Ingress{}
	err = c.client.Put().Namespace(ingress.ObjectMeta.Namespace).Resource(resource).Name(ingress.Name).Body(ingress).Do().Into(result)
	return
}

func (c *Interface) Watch() (watch.Interface, error) {
	opts := &v1.ListOptions{Watch: true}
	return c.client.Get().Resource(resource).VersionedParams(opts, scheme.ParameterCodec).Watch()
}
