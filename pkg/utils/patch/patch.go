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

// Package patch provides utilities for creating JSON patches.
package patch

import (
	"encoding/json"
	"fmt"

	"github.com/evanphx/json-patch"
)

// CreateMergePatch creates a JSON merge patch which transforms the `original`
// to the `modified` object. In addition it returns a boolean flag which is true
// if the patch to be applied is not empty, and an error.
func CreateMergePatch(original, modified interface{}) ([]byte, bool, error) {
	originalBytes, err := json.Marshal(original)
	if err != nil {
		return nil, false, fmt.Errorf("json.Marshal(): %w", err)
	}
	modifiedBytes, err := json.Marshal(modified)
	if err != nil {
		return nil, false, fmt.Errorf("json.Marshal(): %w", err)
	}
	if jsonpatch.Equal(originalBytes, modifiedBytes) {
		return nil, false, nil
	}
	patchBytes, err := jsonpatch.CreateMergePatch(originalBytes, modifiedBytes)
	if err != nil {
		return nil, false, fmt.Errorf("jsonpatch.CreateMergePatch(): %w", err)
	}
	return patchBytes, true, nil
}
