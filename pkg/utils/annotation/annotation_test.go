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

package annotation

import (
	"reflect"
	"testing"

	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testCases = []struct {
	input          string
	output         []string
	existsNonEmpty bool
	desc           string
}{
	{"", nil, false, "Empty annotation"},
	{"xyz", []string{"xyz"}, true, "One element annotation"},
	{" xyz ", []string{"xyz"}, true, "One element annotation with whitespace"},
	{"abc,xyz", []string{"abc", "xyz"}, true, "Multiple element annotation"},
	{" abc , xyz ", []string{"abc", "xyz"}, true, "Multiple element annotation with whitespace"},
}

func newIngress(annotationValue string) *api.Ingress {
	return &api.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{annotation: annotationValue},
		},
	}
}

func TestParseAnnotation(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			output, existsNonEmpty := Parse(newIngress(testCase.input))

			if existsNonEmpty != testCase.existsNonEmpty || !reflect.DeepEqual(output, testCase.output) {
				t.Errorf("Fail: annotation %s expected to be non empty (%t), in fact is (%t). The extracted names expected to be %v, in fact are %v", testCase.input, testCase.existsNonEmpty, existsNonEmpty, testCase.output, output)
			}
		})
	}
}

func TestParseAnnotationMissing(t *testing.T) {
	if _, exists := Parse(&api.Ingress{}); exists {
		t.Errorf("Empty ingress should not have annotation")
	}
}
