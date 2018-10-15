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

package client

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v0.alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/client/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	cloudSdkRootEnv = "CLOUD_SDK_ROOT"
	defaultHost = ""
)

type Clients struct {
	// Mcrt manages ManagedCertificate custom resources
	Mcrt *versioned.Clientset

	// SSL manages SslCertificate GCP resources
	SSL *ssl.SSL
}

func New() (*Clients, error) {
	mcrt, err := getMcrtClient()
	if err != nil {
		return nil, err
	}

	_, err = getComputeClient()
	if err != nil {
		return nil, err
	}

	return &Clients{
		Mcrt: mcrt,
		//SSL: ssl,
	}, nil
}

func (c *Clients) RemoveAll(namespace string) error {
	nsClient := c.Mcrt.GkeV1alpha1().ManagedCertificates(namespace)
	mcrts, err := nsClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mcrt := range mcrts.Items {
		if err := http.IgnoreNotFound(nsClient.Delete(mcrt.Name, &metav1.DeleteOptions{})); err != nil {
			return err
		}
	}

	return nil
}

func getMcrtClient() (*versioned.Clientset, error) {
	kubeConfig := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	c, err := clientcmd.LoadFromFile(kubeConfig)
	if err != nil {
		return nil, err
	}

	overrides := &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: defaultHost}}
	config, err := clientcmd.NewDefaultClientConfig(*c, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}

	return versioned.NewForConfig(config)
}

func getAccessTokenFromGcloud() (string, error) {
	gcloudBin := fmt.Sprintf("%s/bin/gcloud", os.Getenv(cloudSdkRootEnv))
	out, err := exec.Command(gcloudBin, "auth", "print-access-token").Output()
	if err != nil {
		return "", err
	}
	token := strings.Replace(string(out), "\n", "", -1)
	return token, nil
}

func getComputeClient() (*compute.Service, error) {
	accessToken, err := getAccessTokenFromGcloud()
	if err != nil {
		return nil, err
	}
	token := &oauth2.Token{AccessToken: accessToken}
	oauthClient := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))
	return compute.New(oauthClient)
}
