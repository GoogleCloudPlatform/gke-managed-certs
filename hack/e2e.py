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

def kubectl_create(*file_names):
  for file_name in file_names:
    command.call("kubectl create -f {0}/deploy/{1}".format(SCRIPT_ROOT, file_name))

def kubectl_delete(*file_names):
  for file_name in file_names:
    command.call("kubectl delete -f {0}/deploy/{1} --ignore-not-found=true".format(SCRIPT_ROOT, file_name))

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
  names, success = command.call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}'")

  if success:
    for name in names:
      command.call("kubectl delete mcrt {0}".format(name))

def get_firewall_rules():
  uris, _ = command.call_get_out("gcloud compute firewall-rules list --filter=network=e2e --uri 2>/dev/null")
  return uris

def delete_firewall_rules():
  for uri in get_firewall_rules():
    command.call("echo y | gcloud compute firewall-rules delete {0}".format(uri))

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

def init():
  if not os.path.isfile("/etc/service-account/service-account.json"):
    return

  utils.printf("Configure registry authentication")
  command.call("gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json")
  command.call("gcloud auth configure-docker")

  utils.printf("Get kubectl 1.11")
  command.call("curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl")
  command.call("chmod +x kubectl")
  utils.printf("kubectl version: {0}".format(command.call_get_out("./kubectl version")[0][0]))

  utils.printf("Set namespace default")
  command.call("kubectl config set-context $(kubectl config current-context) --namespace=default")

def tearDown(zone):
  utils.printf("Clean up, delete firewall rules, k8s objects, all SslCertificate resources and created DNS records")

  utils.printf("Delete firewall rules for networks matching e2e")
  utils.backoff(delete_firewall_rules, lambda _: len(get_firewall_rules()) == 0)

  kubectl_delete("ingress.yaml", "managed-certificate-controller.yaml")
  delete_managed_certificates()
  kubectl_delete("managedcertificates-crd.yaml", "http-hello.yaml", "rbac.yaml")
  utils.backoff(delete_ssl_certificates, lambda ssl_certificates: len(ssl_certificates) == 0)
  dns.clean_up(zone)

def create_managed_certificates(domains):
  i = 1
  for domain in domains:
    with open("/tmp/managed-certificate.yaml", "w") as f:
      f.write(
"""apiVersion: alpha.cloud.google.com/v1alpha1
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
  utils.printf("Create firewal rules, random DNS records, set up k8s objects")

  instance_prefix = os.getenv("INSTANCE_PREFIX")
  if instance_prefix is not None:
    command_call("gcloud compute firewall-rules create {0}-egress --direction=egress --allow=tcp".format(instance_prefix))
  else:
    utils.printf("INSTANCE_PREFIX env is not set")

  kubectl_create("rbac.yaml", "managedcertificates-crd.yaml", "ingress.yaml", "managed-certificate-controller.yaml")

  domains = dns.create_random_domains(zone)
  create_managed_certificates(domains)

  kubectl_create("http-hello.yaml")

  utils.printf("Expect 2 SslCertificate resources...")
  if utils.backoff(get_ssl_certificates, lambda ssl_certificates: len(ssl_certificates) == 2):
    utils.printf("ok")
  else:
    utils.printf("instead found the following: {0}".format("\n".join(get_ssl_certificates())))

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
