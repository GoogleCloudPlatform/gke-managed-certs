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
	"testing"
)

func TestRandomName_returnsNonEmptyNameShorterThan64Characters(t *testing.T) {
	name, err := RandomName()

	if err != nil {
		t.Errorf("Failed to create random name: %v", err)
	}

	if len(name) <= 0 || len(name) >= 64 {
		t.Errorf("Random name %s has %d characters, should have between 0 and 63", name, len(name))
	}
}

func TestRandomName_calledTwiceReturnsDifferentNames(t *testing.T) {
	name1, err := RandomName()

	if err != nil {
		t.Errorf("Failed to create random name1: %v", err)
	}

	name2, err := RandomName()
	if err != nil {
		t.Errorf("Failed to create random name2 %v", err)
	}

	if name1 == name2 {
		t.Errorf("createRandomName called twice returned the same name %s", name1)
	}
}
