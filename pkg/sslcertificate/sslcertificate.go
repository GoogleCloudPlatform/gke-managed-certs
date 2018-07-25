package sslcertificate

import (
	"cloud.google.com/go/compute/metadata"
	"fmt"
	"github.com/golang/glog"
	gcfg "gopkg.in/gcfg.v1"
	compute "google.golang.org/api/compute/v1"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"os"
	"time"
)

const (
	httpTimeout = 30 * time.Second
)

type SslClient struct {
	service *compute.Service
	projectId string
}

func NewClient(cloudConfig string) (*SslClient, error) {
	tokenSource := google.ComputeTokenSource("")

	if len(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) > 0 {
		tokenSource, err := google.DefaultTokenSource(oauth2.NoContext, compute.ComputeScope)
		if err != nil {
			return nil, err
		}

		glog.V(1).Infof("In a GCP cluster, using TokenSource: %v", tokenSource)
	} else if cloudConfig != "" {
		config, err := os.Open(cloudConfig)
		if err != nil {
			return nil, fmt.Errorf("Could not open cloud provider configuration %s: %#v", cloudConfig, err)
		}
		defer config.Close()

		var cfg gce.ConfigFile
		if err := gcfg.ReadInto(&cfg, config); err != nil {
			return nil, fmt.Errorf("Could not read config %v", err)
		}

		tokenSource := gce.NewAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody)
		glog.V(1).Infof("In a GKE cluster, using TokenSource: %v", tokenSource)
	} else {
		glog.V(1).Infof("Using default TokenSource: %v", tokenSource)
	}

	projectId, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("Could not fetch project id: %v", err)
	}

	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	client.Timeout = httpTimeout

	service, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	return &SslClient{
		service: service,
		projectId: projectId,
	}, nil
}
