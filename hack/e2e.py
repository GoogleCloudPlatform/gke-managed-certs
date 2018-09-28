#!/usr/bin/env python

"""
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

e2e test for Managed Certificates
"""

import argparse
import os
import sys
import urllib2

from utils import command
from utils import dns
from utils import utils

SCRIPT_ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))

def get_ssl_certificates():
  return command.call_get_out("gcloud compute ssl-certificates list --uri")[0]

def delete_managed_certificates():
  utils.printf("Delete ManagedCertificate objects")
  names, success = command.call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}'")

  if success:
    for name in names:
      command.call("kubectl delete mcrt {0}".format(name))

def get_managed_certificate_statuses():
  return command.call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.status.certificateStatus}}{{\"\\n\"}}{{end}}'")[0]

def get_http_statuses(domains):
  statuses = []

  for domain in domains:
    url = "https://{0}".format(domain)
    try:
      connection = urllib2.urlopen(url)
      statuses.append(connection.getcode())
    except Exception, e:
      utils.printf("HTTP GET for {0} failed: {1}".format(url, e))
    finally:
      try:
        connection.close()
      except:
        pass

  return statuses

def expect_ssl_certificates(count):
  utils.printf("Expect {0} SslCertificate resources...".format(count))
  if utils.backoff(get_ssl_certificates, lambda ssl_certificates: len(ssl_certificates) == count):
    utils.printf("ok")
  else:
    utils.printf("instead found the following: {0}".format("\n".join(get_ssl_certificates())))

def create_managed_certificates(domains):
  i = 1
  for domain in domains:
    with open("/tmp/managed-certificate.yaml", "w") as f:
      f.write(
"""apiVersion: gke.googleapis.com/v1alpha1
kind: ManagedCertificate
metadata:
    name: test{0}-certificate
spec:
    domains:
        - {1}
""".format(i, domain))
      f.flush()

    command.call("kubectl create -f /tmp/managed-certificate.yaml", "Deploy test{0}-certificate ManagedCertificate custom object".format(i))
    i += 1

def test(zone):
  utils.printf("Create random DNS records")

  domains = dns.create_random_domains(zone)

  command.call("gcloud alpha compute ssl-certificates create user-created-certificate --global --domains example.com", "Create additional managed SslCertificate to make sure it won't be deleted by managed-certificate-controller")

  create_managed_certificates(domains)

  command.call("kubectl annotate ingress test-ingress gke.googleapis.com/managed-certificates=test1-certificate,test2-certificate")

  expect_ssl_certificates(3)

  utils.printf("Wait for certificates to become Active...")
  if utils.backoff(get_managed_certificate_statuses, lambda statuses: statuses == ["Active", "Active"], max_attempts=50):
    utils.printf("ok")
  else:
    utils.printf("statuses are: {0}. Certificates did not become Active, exiting with failure".format(get_managed_certificate_statuses()))
    sys.exit(1)

  utils.printf("Check HTTP return codes for GET requests to domains {0}...".format(", ".join(domains)))
  if utils.backoff(lambda: get_http_statuses(domains), lambda statuses: statuses == [200, 200]):
    utils.printf("ok")
  else:
    utils.printf("statuses are: {0}. HTTP requests failed, exiting with failure.".format(", ".join(get_http_statuses(domains))))
    sys.exit(1)

  command.call("kubectl delete -f {0}/deploy/ingress.yaml --ignore-not-found=true".format(SCRIPT_ROOT))
  delete_managed_certificates()

  expect_ssl_certificates(1)

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument("--zone", dest="zone", default="managedcertsgke")
  args = parser.parse_args()

  test(args.zone)

if __name__ == '__main__':
  main()
