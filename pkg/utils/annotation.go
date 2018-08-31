/*
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
*/

package utils

import (
	"strings"

	"github.com/golang/glog"
	api "k8s.io/api/extensions/v1beta1"
)

const (
	annotation = "cloud.google.com/managed-certificates"
	splitBy    = ","
)

func ParseAnnotation(ingress *api.Ingress) (result []string, exists bool) {
	annotationValue, exists := ingress.ObjectMeta.Annotations[annotation]
	if !exists {
		return nil, false
	}

	glog.Infof("Found annotation %s on ingress", annotationValue)

	if annotationValue == "" {
		return nil, true
	}

	for _, value := range strings.Split(annotationValue, splitBy) {
		result = append(result, strings.TrimSpace(value))
	}

	return
}
