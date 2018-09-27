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

def create(script_root, *file_names):
  for file_name in file_names:
    command.call("kubectl create -f {0}/deploy/{1}".format(script_root, file_name))

def delete(script_root, *file_names):
  for file_name in file_names:
    command.call("kubectl delete -f {0}/deploy/{1} --ignore-not-found=true".format(script_root, file_name))
