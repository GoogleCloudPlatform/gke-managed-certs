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

// Package errors provides utility functions for manipulating errors.
package errors

import (
	"errors"
	"net/http"

	"google.golang.org/api/googleapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	NotFound = errors.New("Not found")
)

// IsNotFound checks if err is a not found error.
func IsNotFound(err error) bool {
	if errors.Is(err, NotFound) {
		return true
	}

	apiErr, ok := err.(*googleapi.Error)
	if ok && apiErr.Code == http.StatusNotFound {
		return true
	}

	return k8serrors.IsNotFound(err)
}

// IgnoreNotFound returns nil if err is a not found error, else err.
func IgnoreNotFound(err error) error {
	if err == nil {
		return nil
	}

	if IsNotFound(err) {
		return nil
	}

	return err
}
