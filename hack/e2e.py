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
  print("### Remove all existing SslCertificate objects")

  for uri in get_ssl_certificates():
    command.call("echo y | gcloud compute ssl-certificates delete {0}".format(uri))

def get_ssl_certificates():
  return command.call_get_out("gcloud compute ssl-certificates list --uri")[0]

def delete_managed_certificates():
  print("### Delete ManagedCertificate objects")
  names, success = command.call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.metadata.name}}{{\"\\n\"}}{{end}}'")

  if success:
    for name in names:
      command.call("kubectl delete mcrt {0}".format(name))

def get_managed_certificate_statuses():
  return command.call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.status.certificateStatus}}{{\"\\n\"}}{{end}}'")[0]

def init():
  if not os.path.isfile("/etc/service-account/service-account.json"):
    return

  print("### Configure registry authentication")
  command.call("gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json")
  command.call("gcloud auth configure-docker")

  print("### Get kubectl 1.11")
  command.call("curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl")
  command.call("chmod +x kubectl")
  print("### kubectl version: {0}".format(command.call_get_out("./kubectl version")[0][0]))

  print("### Set namespace default")
  command.call("kubectl config set-context $(kubectl config current-context) --namespace=default")

def tearDown(zone):
  print("### Clean up, delete k8s objects, all SslCertificate resources and created DNS records")

  kubectl_delete("ingress.yaml", "managed-certificate-controller.yaml")
  delete_managed_certificates()
  kubectl_delete("managedcertificates-crd.yaml", "http-hello.yaml", "rbac.yaml")
  utils.expBackoff(delete_ssl_certificates, lambda: len(get_ssl_certificates()) == 0)
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
  print("### Create random DNS records, set up k8s objects")

  kubectl_create("rbac.yaml", "managedcertificates-crd.yaml", "ingress.yaml", "managed-certificate-controller.yaml")

  domains = dns.create_random_domains(zone)
  create_managed_certificates(domains)

  kubectl_create("http-hello.yaml")

  print("### expect 2 SslCertificate resources...")
  if utils.expBackoff(lambda: None, lambda: len(get_ssl_certificates()) == 2):
    print("ok")
  else:
    print("instead found the following: {0}".format("\n".join(get_ssl_certificates())))

  print("### wait for certificates to become Active...")
  if utils.expBackoff(lambda: None, lambda: get_managed_certificate_statuses() == ["Active", "Active"], max_attempts=20):
    print("ok")
  else:
    print("statuses are: {0}. Certificates did not become Active, exiting with failure".format(get_managed_certificate_statuses()))
    sys.exit(1)

  for domain in domains:
    try:
      print("### Checking return code for domain {0}".format(domain))
      connection = urllib2.urlopen("https://{0}".format(domain))
      code = connection.getcode()
      if code != 200:
        print("### Code {0} is invalid".format(code))
        sys.exit(1)
      else:
        print("ok")
    finally:
      try:
        connection.close()
      except Exception:
        pass

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
