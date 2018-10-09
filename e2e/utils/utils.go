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

package utils

import (
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
)

const defaultHost = ""

func CreateManagedCertificateClient() (*versioned.Clientset, error) {
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
