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

Functions operating on DNS records in Cloud DNS
"""

import command
import os.path
import random
import string
import time
import utils

RECORD_LENGTH = 20
PROJECT = "certsbridge-dev"

def create_random_domains(zone_name):
  """
  Generates 2 random domain names in zone_name, under top-level domain com.certsbridge
  """

  output, _ = command.call_get_out("gcloud compute addresses describe test-ip-address --global | grep address: | cut -d ' ' -f 2")
  ip = output[0]
  utils.printf("Creating random domains pointing at ip {0}".format(ip))

  command.call("gcloud dns record-sets transaction start --zone {0} --project {1}".format(zone_name, PROJECT))

  result = []

  for i in range(2):
    record = ''.join(random.choice(string.ascii_lowercase) for _ in range(RECORD_LENGTH))
    domain = "{0}.{1}.certsbridge.com".format(record, zone_name)
    result.append(domain)

    command.call("gcloud dns record-sets transaction add --zone {0} --project {1} --name='{2}' --type=A --ttl=300 {3}".format(zone_name, PROJECT, domain, ip), "Add DNS record for domain {0} to ip {1}".format(domain, ip))

  command.call("gcloud dns record-sets transaction execute --zone {0} --project {1}".format(zone_name, PROJECT))

  return result
