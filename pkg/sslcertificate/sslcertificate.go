package sslcertificate

import (
	"cloud.google.com/go/compute/metadata"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/uuid"
	gcfg "gopkg.in/gcfg.v1"
	compute "google.golang.org/api/compute/v0.alpha"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"os"
	"time"
)

const (
	httpTimeout = 30 * time.Second
	maxNameLength = 63
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

func (c *SslClient) Delete(name string) error {
	_, err := c.service.SslCertificates.Delete(c.projectId, name).Do()
	return err
}

func (c *SslClient) Get(name string) (*compute.SslCertificate, error) {
	return c.service.SslCertificates.Get(c.projectId, name).Do()
}

func (c *SslClient) Insert(domains []string) (string, error) {
	sslCertificateName, err := createRandomName()

	if err != nil {
		return "", err
	}

	_, err = c.Get(sslCertificateName)
	if err != nil {
		// Name taken, choose a new one
		sslCertificateName, err = createRandomName()

		if err != nil {
			return "", err
		}
	}

	sslCertificate := &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: domains,
		},
		Name: sslCertificateName,
		Type: "MANAGED",
	}

	_, err = c.service.SslCertificates.Insert(c.projectId, sslCertificate).Do()
	if err != nil {
		return "", fmt.Errorf("Failed to insert SslCertificate %v, err: %v", sslCertificate, err)
	}

	return sslCertificateName, nil
}

func (c *SslClient) List() (*compute.SslCertificateList, error) {
	return c.service.SslCertificates.List(c.projectId).Do()
}

func createRandomName() (string, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("mcert%s", uid.String())[:maxNameLength], nil
}
