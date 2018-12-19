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

// Package config manages configuration of the whole application.
package config

import (
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	gcfg "gopkg.in/gcfg.v1"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

const (
	SslCertificateNamePrefix = "mcrt-"
)

type computeConfig struct {
	TokenSource oauth2.TokenSource
	ProjectID   string
	Timeout     time.Duration
}

type Config struct {
	Compute                  computeConfig
	SslCertificateNamePrefix string
}

func New(gceConfigFilePath string) (*Config, error) {
	tokenSource, projectID, err := getTokenSourceAndProjectID(gceConfigFilePath)
	if err != nil {
		return nil, err
	}

	glog.Infof("TokenSource: %#v, projectID: %s", tokenSource, projectID)

	return &Config{
		Compute: computeConfig{
			TokenSource: tokenSource,
			ProjectID:   projectID,
			Timeout:     30 * time.Second,
		},
		SslCertificateNamePrefix: SslCertificateNamePrefix,
	}, nil
}

func getTokenSourceAndProjectID(gceConfigFilePath string) (oauth2.TokenSource, string, error) {
	if gceConfigFilePath != "" {
		glog.V(1).Info("In a GKE cluster")

		config, err := os.Open(gceConfigFilePath)
		if err != nil {
			return nil, "", fmt.Errorf("Could not open cloud provider configuration %s: %v", gceConfigFilePath, err)
		}
		defer config.Close()

		var cfg gce.ConfigFile
		if err := gcfg.ReadInto(&cfg, config); err != nil {
			return nil, "", fmt.Errorf("Could not read config %v", err)
		}
		glog.Infof("Using GCE provider config %+v", cfg)

		return gce.NewAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody), cfg.Global.ProjectID, nil
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, "", fmt.Errorf("Could not fetch project id: %v", err)
	}

	if len(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) > 0 {
		glog.V(1).Info("In a GCP cluster")
		tokenSource, err := google.DefaultTokenSource(oauth2.NoContext, compute.ComputeScope)
		return tokenSource, projectID, err
	} else {
		glog.V(1).Info("Using default TokenSource")
		return google.ComputeTokenSource(""), projectID, nil
	}
}
