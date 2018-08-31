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
import utils

RECORD_LENGTH = 20

def switch_to_certsbridge_conf():
  configurations, success = command.call_get_out("gcloud config configurations list --filter=name=certsbridge")
  configuration_exists = len(configurations) > 0

  if not configuration_exists:
    command.call("gcloud config configurations create certsbridge", "Create gcloud conf certsbridge")
    command.call("gcloud config set compute/zone europe-west1-b")
    command.call("gcloud config set project certsbridge-dev")

    if os.path.isfile("/etc/service-account/service-account.json"):
      command.call("gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json")
  else:
    command.call("gcloud config configurations activate certsbridge", "Switch gcloud conf to certsbridge")

def switch_to_default_conf():
  command.call("gcloud config configurations activate default", "Switch gcloud conf to default")

def clean_up(zone_name):
  clear_dns_zone(zone_name)

  if os.path.isfile("/etc/service-account/service-account.json"):
    clear_conf()

def clear_conf():
  command.call("echo y | gcloud config configurations delete certsbridge", "Remove gcloud conf certsbridge")

def clear_dns_zone(zone_name):
  """
  Removes A sub-records of com.certsbridge from a given DNS zone.
  """
  utils.printf("Removing all sub-records of com.certsbridge from zone {0}".format(zone_name))
  switch_to_certsbridge_conf()

  output = command.call_get_out("gcloud dns record-sets list --zone {0} --filter=type=A | grep certsbridge.com | tr -s ' ' | cut -d ' ' -f 1,4".format(zone_name))[0]

  command.call("gcloud dns record-sets transaction start --zone {0}".format(zone_name))
  for line in output:
    domain, ip = line.split(" ")
    command.call("gcloud dns record-sets transaction remove --zone {0} --name='{1}' --type=A --ttl=300 {2}".format(zone_name, domain, ip), "Remove DNS record for domain {0}".format(domain))

  command.call("gcloud dns record-sets transaction execute --zone {0}".format(zone_name))

  switch_to_default_conf()

def create_random_domains(zone_name):
  """
  Generates 2 random domain names in zone_name, under top-level domain com.certsbridge
  """

  output, _ = command.call_get_out("gcloud compute addresses describe test-ip-address --global | grep address: | cut -d ' ' -f 2")
  ip = output[0]
  utils.printf("Creating random domains pointing at ip {0}".format(ip))

  switch_to_certsbridge_conf()

  command.call("gcloud dns record-sets transaction start --zone {0}".format(zone_name))

  result = []

  for i in range(2):
    record = ''.join(random.choice(string.ascii_lowercase) for _ in range(RECORD_LENGTH))
    domain = "{0}.{1}.certsbridge.com".format(record, zone_name)
    result.append(domain)

    command.call("gcloud dns record-sets transaction add --zone {0} --name='{1}' --type=A --ttl=300 {2}".format(zone_name, domain, ip), "Add DNS record for domain {0} to ip {1}".format(domain, ip))

  command.call("gcloud dns record-sets transaction execute --zone {0}".format(zone_name))

  switch_to_default_conf()

  return result
