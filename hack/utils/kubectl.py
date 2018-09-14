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

Wrapper around kubectl
"""

import command
import os.path
import utils

PROW_TEST = os.path.isfile("/etc/service-account/service-account.json")
KUBECTL_NAME = "kubectl1_11" if PROW_TEST else "kubectl"

def call(com, info=None):
  return command.call("{0} {1}".format(KUBECTL_NAME, com), info)

def call_get_out(com):
  return command.call_get_out("{0} {1}".format(KUBECTL_NAME, com))

def create(script_root, *file_names):
  for file_name in file_names:
    call("create -f {0}/deploy/{1}".format(script_root, file_name))

def delete(script_root, *file_names):
  for file_name in file_names:
    call("delete -f {0}/deploy/{1} --ignore-not-found=true".format(script_root, file_name))

def install():
  if not PROW_TEST:
    return

  utils.printf("Get kubectl 1.11")
  command.call("curl -L -o {0} https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl".format(KUBECTL_NAME))
  command.call("chmod +x {0}".format(KUBECTL_NAME))
  utils.printf("kubectl version: {0}".format(call_get_out("version")[0][0]))
