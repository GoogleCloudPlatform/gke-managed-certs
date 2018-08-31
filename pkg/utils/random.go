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
	"fmt"

	"github.com/google/uuid"
)

const (
	maxNameLength = 63
)

func min(a, b int) int { // golang style recommends just inlining this code
	if a < b {
		return a
	}

	return b
}

func RandomName() (string, error) { // if you are using random, seed it from the command line flags for repeatability
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	generatedName := fmt.Sprintf("mcert%s", uid.String())
	maxLength := min(len(generatedName), maxNameLength)
	return generatedName[:maxLength], nil
}
