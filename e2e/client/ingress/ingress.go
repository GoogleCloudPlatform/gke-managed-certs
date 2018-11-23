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

package ingress

import (
	api "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/rest"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

type Ingress interface {
	Create(namespace, name string) error
	Delete(namespace, name string) error
	Get(namespace, name string) (*api.Ingress, error)
	Update(ing *api.Ingress) error
}

type ingressImpl struct {
	// getter manages Kubernetes Ingress objects
	getter v1beta1.IngressesGetter
}

func New(config *rest.Config) (Ingress, error) {
	getter, err := v1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return ingressImpl{
		getter: getter,
	}, nil
}

func (i ingressImpl) Create(namespace, name string) error {
	ing := &api.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: api.IngressSpec{
			Backend: &api.IngressBackend{
				ServiceName: "http-hello",
				ServicePort: intstr.FromInt(8080),
			},
		},
	}
	_, err := i.getter.Ingresses(namespace).Create(ing)
	return err
}

func (i ingressImpl) Delete(namespace, name string) error {
	return http.IgnoreNotFound(i.getter.Ingresses(namespace).Delete(name, &metav1.DeleteOptions{}))
}

func (i ingressImpl) Get(namespace, name string) (*api.Ingress, error) {
	return i.getter.Ingresses(namespace).Get(name, metav1.GetOptions{})
}

func (i ingressImpl) Update(ing *api.Ingress) error {
	_, err := i.getter.Ingresses(ing.Namespace).Update(ing)
	return err
}
