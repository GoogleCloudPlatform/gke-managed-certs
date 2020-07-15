/*
Copyright 2020 Google LLC

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

package e2e

import (
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func createIngress(t *testing.T, name string, port int32, annotationManagedCertificatesValue string) error {
	t.Helper()

	if err := http.IgnoreNotFound(clients.Deployment.Delete(name, &metav1.DeleteOptions{})); err != nil {
		return err
	}

	appHello := map[string]string{"app": name}
	args := []string{
		"-e",
		fmt.Sprintf("require('http').createServer(function (req, res) { res.end('Hello world!'); }).listen(%d);", port),
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: appHello},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: appHello},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:    "http-hello",
							Image:   "node:11-slim",
							Command: []string{"node"},
							Args:    args,
							Ports:   []corev1.ContainerPort{{ContainerPort: port}},
						},
					},
				},
			},
		},
	}
	if _, err := clients.Deployment.Create(deployment); err != nil {
		return err
	}
	t.Cleanup(func() { clients.Deployment.Delete(name, &metav1.DeleteOptions{}) })

	if err := http.IgnoreNotFound(clients.Service.Delete(name, &metav1.DeleteOptions{})); err != nil {
		return err
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Ports:    []corev1.ServicePort{{Port: port}},
			Selector: appHello,
		},
	}
	if _, err := clients.Service.Create(service); err != nil {
		return err
	}
	t.Cleanup(func() { clients.Service.Delete(name, &metav1.DeleteOptions{}) })

	if err := http.IgnoreNotFound(clients.Ingress.Delete(name, &metav1.DeleteOptions{})); err != nil {
		return err
	}

	ingress := &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"networking.gke.io/managed-certificates": annotationManagedCertificatesValue,
			},
		},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: name,
				ServicePort: intstr.FromInt(int(port)),
			},
		},
	}
	if _, err := clients.Ingress.Create(ingress); err != nil {
		return err
	}
	t.Cleanup(func() { clients.Ingress.Delete(name, &metav1.DeleteOptions{}) })

	return nil
}
