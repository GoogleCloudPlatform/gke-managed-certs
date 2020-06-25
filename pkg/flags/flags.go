/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package flags defines global controller flags.
package flags

import (
	"flag"
)

var (
	F = struct {
		APIServerHost      string
		GCEConfigFilePath  string
		KubeConfigFilePath string
		PrometheusAddress  string
	}{}
)

// Register registers flags with command line parser.
func Register() {
	flag.StringVar(&F.APIServerHost, "apiserver-host", "",
		`The address of the Kubernetes Apiserver to connect to in the format of
protocol://address:port, e.g., http://localhost:8080. If not specified, the
assumption is that the binary runs inside a Kubernetes cluster and local
discovery is attempted.`)
	flag.StringVar(&F.GCEConfigFilePath, "gce-config-file-path", "",
		"Path to a file containing the gce config.")
	flag.StringVar(&F.KubeConfigFilePath, "kube-config-file-path", "",
		"Path to kubeconfig file with authorization and master location information.")
	flag.StringVar(&F.PrometheusAddress, "prometheus-address", ":8910",
		"The address to expose Prometheus metrics")

	flag.Parse()
}
