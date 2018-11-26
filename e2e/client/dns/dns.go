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

package dns

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
	dns "google.golang.org/api/dns/v2beta1"
)

const (
	projectID    = "certsbridge-dev"
	recordTypeA  = "A"
	topLevelZone = "certsbridge.com"
)

type Dns interface {
	Create(randomNames []string, ip string) ([]string, error)
	DeleteAll() error
}

type dnsImpl struct {
	// service provides operations on DNS resources
	service *dns.Service
	// zone is a DNS zone this client operates on
	zone string
}

func New(oauthClient *http.Client, zone string) (Dns, error) {
	service, err := dns.New(oauthClient)
	if err != nil {
		return nil, err
	}

	return dnsImpl{
		service: service,
		zone:    zone,
	}, nil
}

// Create for each item in randomNames adds A record to DNS zone {d.zone}.{topLevelZone} and returns resulting domain names.
func (d dnsImpl) Create(randomNames []string, ip string) ([]string, error) {
	var domainNames []string
	var additions []*dns.ResourceRecordSet

	for _, randomName := range randomNames {
		domainName := fmt.Sprintf("%s.%s.%s", randomName, d.zone, topLevelZone)
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
	return domainNames, err
}

// DeleteAll deletes all A records in DNS zone {d.zone}.{topLevelZone}.
func (d dnsImpl) DeleteAll() error {
	resourceRecordsResponse, err := d.service.ResourceRecordSets.List(projectID, d.zone).Do()
	if err != nil {
		return err
	}

	var allNames []string
	var allANames []string
	var recordsA []*dns.ResourceRecordSet
	for _, record := range resourceRecordsResponse.Rrsets {
		allNames = append(allNames, record.Name)

		if record.Type == recordTypeA {
			recordsA = append(recordsA, record)
			allANames = append(allANames, record.Name)
		}
	}

	glog.Infof("Delete all DNS A records in %s.%s; all names: %v; all A names: %v", d.zone, topLevelZone, allNames, allANames)

	_, err = d.service.Changes.Create(projectID, d.zone, &dns.Change{
		Deletions: recordsA,
	}).Do()

	glog.Infof("Successfully deleted DNS A records: %v", allANames)
	return err
}
