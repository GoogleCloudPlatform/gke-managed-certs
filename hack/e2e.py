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
from utils import kubectl
from utils import utils

SCRIPT_ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
PROW_TEST = os.path.isfile("/etc/service-account/service-account.json")

def delete_ssl_certificates():
  utils.printf("Remove all existing SslCertificate objects")

  ssl_certificates = get_ssl_certificates()

  for uri in ssl_certificates:
    command.call("echo y | gcloud compute ssl-certificates delete {0}".format(uri))

  return ssl_certificates

def get_ssl_certificates():
  return command.call_get_out("gcloud compute ssl-certificates list --uri")[0]

def delete_managed_certificates():
  utils.printf("Delete ManagedCertificate objects")
  names, success = kubectl.call_get_out("get mcrt -o go-template='{{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}'")

  if success:
    for name in names:
      kubectl.call("delete mcrt {0}".format(name))

def get_firewall_rules():
  uris, _ = command.call_get_out("gcloud compute firewall-rules list --filter='network=e2e AND name=mcrt' --uri 2>/dev/null")
  return uris

def delete_firewall_rules():
  for uri in get_firewall_rules():
    command.call("echo y | gcloud compute firewall-rules delete {0}".format(uri))

def get_managed_certificate_statuses():
  return kubectl.call_get_out("get mcrt -o go-template='{{range .items}}{{.status.certificateStatus}}{{\"\\n\"}}{{end}}'")[0]

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

def init():
  if not PROW_TEST:
    return

  utils.printf("Configure registry authentication")
  command.call("gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json")
  command.call("gcloud auth configure-docker")

  kubectl.install()

  utils.printf("Set namespace default")
  current_context = kubectl.call_get_out("config current-context")[0]
  kubectl.call("config set-context {0} --namespace=default".format(current_context))

def tearDown(zone):
  utils.printf("Clean up, delete firewall rules, k8s objects, all SslCertificate resources and created DNS records")

  utils.printf("Delete firewall rules for networks matching e2e")
  utils.backoff(delete_firewall_rules, lambda _: len(get_firewall_rules()) == 0)

  kubectl.delete(SCRIPT_ROOT, "ingress.yaml", "managed-certificate-controller.yaml")
  delete_managed_certificates()
  kubectl.delete(SCRIPT_ROOT, "managedcertificates-crd.yaml", "http-hello.yaml", "rbac.yaml")
  utils.backoff(delete_ssl_certificates, lambda ssl_certificates: len(ssl_certificates) == 0)
  dns.clean_up(zone)

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

    kubectl.call("create -f /tmp/managed-certificate.yaml", "Deploy test{0}-certificate ManagedCertificate custom object".format(i))
    i += 1

def test(zone):
  utils.printf("Create firewall rules, random DNS records, set up k8s objects")

  instance_prefix = os.getenv("INSTANCE_PREFIX")
  if instance_prefix is not None:
    command.call("gcloud compute firewall-rules create mcrt-{0} --network={0} --allow=tcp,udp,icmp,esp,ah,sctp".format(instance_prefix))
  else:
    utils.printf("INSTANCE_PREFIX env is not set")

  kubectl.create(SCRIPT_ROOT, "rbac.yaml", "managedcertificates-crd.yaml", "ingress.yaml")

  domains = dns.create_random_domains(zone)

  command.call("gcloud alpha compute ssl-certificates create user-created-certificate --global --domains example.com", "Create additional managed SslCertificate to make sure it won't be deleted by managed-certificate-controller")

  create_managed_certificates(domains)

  kubectl.create(SCRIPT_ROOT, "managed-certificate-controller.yaml", "http-hello.yaml")

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

  kubectl.call("annotate ingress test-ingress gke.googleapis.com/managed-certificates-", "Remove managed-certificates annotation")
  kubectl.call("annotate ingress test-ingress ingress.gcp.kubernetes.io/pre-shared-cert-", "Remove pre-shared-cert annotation")
  kubectl.delete(SCRIPT_ROOT, "ingress.yaml")
  delete_managed_certificates()

  expect_ssl_certificates(1)

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument("--zone", dest="zone", default="managedcertsgke")
  args = parser.parse_args()

  init()

  tearDown(args.zone)
  test(args.zone)
  tearDown(args.zone)

if __name__ == '__main__':
  main()
