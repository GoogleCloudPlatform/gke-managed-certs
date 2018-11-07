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

	"github.com/golang/glog"
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
	projectIDEnv = "PROJECT_ID"
)

type managedCertificate struct {
	// clientset manages ManagedCertificate custom resources
	clientset versioned.Interface
}

type sslCertificate struct {
	// sslCertificates manages GCP SslCertificate resources
	sslCertificates *compute.SslCertificatesService

	// projectID is the id of the project in which e2e tests are run
	projectID string
}

type Clients struct {
	ManagedCertificate managedCertificate
	SslCertificate     sslCertificate
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

	projectID := os.Getenv(projectIDEnv)
	glog.Infof("projectID=%s", projectID)

	return &Clients{
		ManagedCertificate: managedCertificate{
			clientset: clientset,
		},
		SslCertificate: sslCertificate{
			sslCertificates: computeClient.SslCertificates,
			projectID:       projectID,
		},
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
	gcloudAuthList, err := gcloud("auth", "list")
	if err != nil {
		return nil, err
	}
	glog.Infof("gcloud auth list: %s", gcloudAuthList)

	gcloudInfo, err := gcloud("info")
	if err != nil {
		return nil, err
	}
	glog.Infof("gcloud info: %s", gcloudInfo)

	gcloudConfigurations, err := gcloud("config", "configurations", "list")
	if err != nil {
		return nil, err
	}
	glog.Infof("gcloud config configurations list: %s", gcloudConfigurations)

	accessToken, err := gcloud("auth", "print-access-token")
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{AccessToken: accessToken}
	oauthClient := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))
	return compute.New(oauthClient)
}
