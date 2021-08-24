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

// Package dns provides operations needed to interact with DNS in an e2e test.
package dns

import (
	"fmt"
	"net/http"

	dns "google.golang.org/api/dns/v2beta1"
	"k8s.io/klog"
)

const (
	projectID   = "gke-net-dns"
	recordTypeA = "A"
)

type Interface interface {
	// Create adds DNS A records pointing `randomNames` at the IP address to the configured DNS zone
	// and returns the resulting domain names and the records added to the zone.
	Create(randomNames []string, ip string) ([]string, []*dns.ResourceRecordSet, error)
	// Delete deletes specified records from the DNS zone `d.zone`.
	Delete(records []*dns.ResourceRecordSet) error
}

type impl struct {
	// service provides operations on DNS resources
	service *dns.Service
	// zone is a DNS zone this client operates on
	zone string
	// domain is a DNS name part under which random entries are generated
	domain string
}

func New(oauthClient *http.Client, zone, domain string) (Interface, error) {
	service, err := dns.New(oauthClient)
	if err != nil {
		return nil, err
	}

	return impl{
		domain:  domain,
		service: service,
		zone:    zone,
	}, nil
}

// Create adds DNS A records pointing `randomNames` at the IP address `ip` to the configured DNS zone.
//
// For each item `randomName` in `randomNames` an A record is added to the `d.zone` DNS zone.
// The record points `randomName`.`d.domain` to the `ip` address.
//
// Returns the resulting domain names and DNS records added to the DNS zone.
func (d impl) Create(randomNames []string, ip string) ([]string, []*dns.ResourceRecordSet, error) {
	var domainNames []string
	var additions []*dns.ResourceRecordSet

	for _, randomName := range randomNames {
		domainName := fmt.Sprintf("%s.%s", randomName, d.domain)
		domainNames = append(domainNames, domainName)
		additions = append(additions, &dns.ResourceRecordSet{
			Name:    fmt.Sprintf("%s.", domainName),
			Rrdatas: []string{ip},
			Ttl:     20,
			Type:    recordTypeA,
		})
	}

	_, err := d.service.Changes.Create(projectID, d.zone, &dns.Change{
		Additions: additions,
	}).Do()
	return domainNames, additions, err
}

// Delete deletes specified records from the DNS zone `d.zone`.
func (d impl) Delete(records []*dns.ResourceRecordSet) error {
	var names []string
	for _, record := range records {
		names = append(names, record.Name)
	}

	klog.Infof("Delete DNS records from %s; names: %v", d.zone, names)

	_, err := d.service.Changes.Create(projectID, d.zone, &dns.Change{
		Deletions: records,
	}).Do()
	return err
}
