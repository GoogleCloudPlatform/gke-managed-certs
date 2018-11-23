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
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client/dns"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client/sslcertificate"
)

const (
	cloudSdkRootEnv = "CLOUD_SDK_ROOT"
	defaultHost     = ""
	dnsZoneEnv      = "DNS_ZONE"
	projectIDEnv    = "PROJECT_ID"
)

type Clients struct {
	Dns                dns.Dns
	Ingress            ingress.Ingress
	ManagedCertificate managedcertificate.ManagedCertificate
	SslCertificate     sslcertificate.SslCertificate
}

func New() (*Clients, error) {
	config, err := getRestConfig()
	if err != nil {
		return nil, err
	}

	ingressClient, err := ingress.New(config)
	if err != nil {
		return nil, err
	}

	managedCertificateClient, err := managedcertificate.New(config)
	if err != nil {
		return nil, err
	}

	oauthClient, err := getOauthClient()
	if err != nil {
		return nil, err
	}

	projectID := os.Getenv(projectIDEnv)
	glog.Infof("projectID=%s", projectID)

	dnsZone := os.Getenv(dnsZoneEnv)
	glog.Infof("dnsZone=%s", dnsZone)

	dnsClient, err := dns.New(oauthClient, dnsZone)
	if err != nil {
		return nil, err
	}

	sslCertificateClient, err := sslcertificate.New(oauthClient, projectID)
	if err != nil {
		return nil, err
	}

	return &Clients{
		Dns:                dnsClient,
		Ingress:            ingressClient,
		ManagedCertificate: managedCertificateClient,
		SslCertificate:     sslCertificateClient,
	}, nil
}

func getRestConfig() (*rest.Config, error) {
	kubeConfig := os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	c, err := clientcmd.LoadFromFile(kubeConfig)
	if err != nil {
		return nil, err
	}

	overrides := &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: defaultHost}}
	return clientcmd.NewDefaultClientConfig(*c, overrides).ClientConfig()
}

func gcloud(command ...string) (string, error) {
	gcloudBin := fmt.Sprintf("%s/bin/gcloud", os.Getenv(cloudSdkRootEnv))
	out, err := exec.Command(gcloudBin, command...).Output()
	if err != nil {
		return "", err
	}
	return strings.Replace(string(out), "\n", "", -1), nil
}

func getOauthClient() (*http.Client, error) {
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
	return oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token)), nil
}
