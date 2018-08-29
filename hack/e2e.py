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
import subprocess
import sys
import time

SCRIPT_ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))

def call(command):
  subprocess.call(command, shell=True)

def call_get_out(command):
  p = subprocess.Popen(command, shell=True, stdout=subprocess.PIPE)
  return p.communicate()[0]

def kubectl_create(file_name):
  call("kubectl create -f {0}/deploy/{1}".format(SCRIPT_ROOT, file_name))

def kubectl_delete(file_name):
  call("kubectl delete -f {0}/deploy/{1} --ignore-not-found=true".format(SCRIPT_ROOT, file_name))

def delete_ssl_certificates():
  for uri in get_ssl_certificates():
    call("echo y | gcloud compute ssl-certificates delete {0}".format(uri))

def get_ssl_certificates():
  return filter(None, call_get_out("gcloud compute ssl-certificates list --uri").split("\n"))

def get_managed_certificate_statuses():
  return call_get_out("kubectl get mcrt -o go-template='{{range .items}}{{.status.certificateStatus}}{{\":\"}}{{end}}'")

def expBackoff(action, condition, max_attempts=10):
  timeout = 1

  for attempt in range(max_attempts):
    action()

    if condition():
      return True

    print("### Condition not met, retrying in {0} seconds...".format(timeout))
    time.sleep(timeout)
    timeout *= 2

  return False

def init():
  print("### Configure registry authentication")
  call("gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json")
  call("gcloud auth configure-docker")

  print("### get kubectl 1.11")
  call("curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl")
  call("chmod +x kubectl")
  print("### kubectl version: {0}".format(call_get_out("./kubectl version")))

  print("### set namespace default")
  call("kubectl config set-context $(kubectl config current-context) --namespace=default")

def tearDown():
  print("### Delete managed-certificate-controller")
  kubectl_delete("managed-certificate-controller.yaml")

  print("### Delete CRD")
  kubectl_delete("managedcertificates-crd.yaml")

  print("### Delete ingress")
  kubectl_delete("ingress.yaml")

  print("### Delete http-hello service")
  kubectl_delete("http-hello.yaml")

  print("### Remove RBAC")
  kubectl_delete("rbac.yaml")

  print("### Remove all existing SslCertificate objects")
  expBackoff(delete_ssl_certificates, lambda: len(get_ssl_certificates()) == 0)

def setUp():
  print("### Deploy RBAC")
  kubectl_create("rbac.yaml")

  print("### Deploy CRD")
  kubectl_create("managedcertificates-crd.yaml")

  print("### Deploy managed-certificate-controller")
  kubectl_create("managed-certificate-controller.yaml")

  print("### Deploy test1-certificate and test2-certificate ManagedCertificate custom objects")
  kubectl_create("test1-certificate.yaml")
  kubectl_create("test2-certificate.yaml")

  print("### Deploy http-hello service")
  kubectl_create("http-hello.yaml")

  print("### Deploy ingress")
  kubectl_create("ingress.yaml")

def test():
  print("### expect 2 SslCertificate resources...")
  if expBackoff(lambda: None, lambda: len(get_ssl_certificates()) == 2):
    print("ok")
  else:
    print("instead found the following: {0}".format("\n".join(get_ssl_certificates())))

  print("### wait for certificates to become Active...")
  if expBackoff(lambda: None, lambda: get_managed_certificate_statuses() == "Active:Active:", max_attempts=20):
    print("ok")
  else:
    print("statuses are: {0}. Certificates did not become Active, exiting with failure".format(get_managed_certificate_statuses()))
    sys.exit(1)

  print("### remove annotation managed-certificates from ingress")
  call("kubectl annotate ingress test-ingress cloud.google.com/managed-certificates-")

  print("### remove custom resources test1-certificate and test2-certificate")
  kubectl_delete("test1-certificate.yaml")
  kubectl_delete("test2-certificate.yaml")

  print("### Remove all existing SslCertificate objects")
  expBackoff(delete_ssl_certificates, lambda: len(get_ssl_certificates()) == 0)

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument("--noinit", dest="init", action="store_false")
  args = parser.parse_args()

  if args.init:
    init()

  tearDown()
  setUp()

  test()

  tearDown()

if __name__ == '__main__':
  main()
