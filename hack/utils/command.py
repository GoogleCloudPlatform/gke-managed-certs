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

Wrappers around subprocess which simplify calling external commands from Python scripts.
"""

import subprocess

def call(command, info=None):
  """
  Calls a command through shell
  """
  output, success = call_get_out(command, info)

def call_get_out(command, info=None):
  """
  Calls a command through shell and returns a tuple which:
    * first element is a list of output lines with empty lines removed
    * second element is a flag indicating whether the command succeeded or not
  """
  if info is not None:
    print("### {0}".format(info))

  #print("### Executing $ {0}".format(command))
  p = subprocess.Popen(command, shell=True, stdout=subprocess.PIPE)
  output = filter(None, p.communicate()[0].split("\n"))
  return (output, p.returncode == 0)
