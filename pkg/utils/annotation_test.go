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
	api "k8s.io/api/extensions/v1beta1"
        "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func newIngress(annotationValue string) *api.Ingress {
	return &api.Ingress {
		ObjectMeta: v1.ObjectMeta {
			Annotations: map[string]string{annotation: annotationValue},
		},
	}
}

func TestParseAnnotation_annotationMissing(t *testing.T) {
	_, exists := ParseAnnotation(&api.Ingress{})

	if exists {
		t.Errorf("Empty ingress should not have annotation")
	}
}

func TestParseAnnotation_empty(t *testing.T) {
	names, exists := ParseAnnotation(newIngress(""))

	if !exists {
		t.Errorf("Annotation should be present")
	}

	if len(names) != 0 {
		t.Errorf("Empty value annotation \"\" should be parsed into empty array, is instead: %v.", names)
	}
}

func TestParseAnnotation_oneElement(t *testing.T) {
	names, exists := ParseAnnotation(newIngress("xyz"))

	if !exists {
		t.Errorf("Annotation should be present")
	}

	if len(names) != 1 || names[0] != "xyz" {
		t.Errorf("One element annotation \"xyz\" should be parsed into one element array with value xyz, is instead: %v.", names)
	}
}

func TestParseAnnotation_oneElementWithWhitespace(t *testing.T) {
	names, exists := ParseAnnotation(newIngress(" xyz "))

	if !exists {
		t.Errorf("Annotation should be present")
	}

	if len(names) != 1 || names[0] != "xyz" {
		t.Errorf("One element annotation with whitespace \" xyz \" should be parsed into one element array with value xyz, is instead: %v.", names)
	}
}

func TestParseAnnotation_multiElement(t *testing.T) {
	names, exists := ParseAnnotation(newIngress("xyz,abc"))

	if !exists {
		t.Errorf("Annotation should be present")
	}

	if len(names) != 2 || names[0] != "xyz" || names[1] != "abc" {
		t.Errorf("Multi element annotation \"xyz,abc\" should be parsed into two element array with values xyz, abc, is instead: %v.", names)
	}
}

func TestParseAnnotation_multiElementWithWhitespace(t *testing.T) {
	names, exists := ParseAnnotation(newIngress(" xyz, abc "))

	if !exists {
		t.Errorf("Annotation should be present")
	}

	if len(names) != 2 || names[0] != "xyz" || names[1] != "abc" {
		t.Errorf("Multi element annotation with whitespace \" xyz , abc \" should be parsed into two element array with values xyz, abc, is instead: %v.", names)
	}
}
