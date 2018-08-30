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

Utils for e2e test for Managed Certificates
"""

import time

def expBackoff(action, condition, max_attempts=10):
  """
  Calls action() up to max_attempts times until condition() becomes true, with exponential backoff. Returns a bool flag indicating whether action() succeded.
  """
  timeout = 1

  for attempt in range(max_attempts):
    action()

    if condition():
      return True

    print("### Condition not met, retrying in {0} seconds...".format(timeout))
    time.sleep(timeout)
    timeout *= 2

  return False
