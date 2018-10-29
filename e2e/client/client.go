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
	compute "google.golang.org/api/compute/v0.beta"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
)

const (
	cloudSdkRootEnv = "CLOUD_SDK_ROOT"
	defaultHost     = ""
)

type Clients struct {
	// Clientset manages ManagedCertificate custom resources
	Clientset versioned.Interface

	// Compute manages GCP resources
	Compute *compute.Service

	// ProjectID is the id of the project in which e2e tests are run
	ProjectID string
}

func New() (*Clients, error) {
	clientset, err := getMcrtClient()
	if err != nil {
		return nil, err
	}

	computeClient, err := getComputeClient()
	if err != nil {
		return nil, err
	}

	projectID, err := gcloud("config", "list", "--format=value(core.project)")
	if err != nil {
		return nil, err
	}

	return &Clients{
		Clientset: clientset,
		Compute:   computeClient,
		ProjectID: projectID,
	}, nil
}

func getMcrtClient() (versioned.Interface, error) {
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

func gcloud(command ...string) (string, error) {
	gcloudBin := fmt.Sprintf("%s/bin/gcloud", os.Getenv(cloudSdkRootEnv))
	out, err := exec.Command(gcloudBin, command...).Output()
	if err != nil {
		return "", err
	}
	return strings.Replace(string(out), "\n", "", -1), nil
}

func getComputeClient() (*compute.Service, error) {
	accessToken, err := gcloud("auth", "print-access-token")
	if err != nil {
		return nil, err
	}
	token := &oauth2.Token{AccessToken: accessToken}
	oauthClient := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))
	return compute.New(oauthClient)
}
