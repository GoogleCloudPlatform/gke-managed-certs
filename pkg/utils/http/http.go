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

// Package http provides utility functions for manipulating HTTP errors.
package http

import (
	"net/http"

	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	quotaExceeded = "quotaExceeded"
)

// IsNotFound checks if err is an HTTP Not Found (404) error.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if ok && apiErr.Code == http.StatusNotFound {
		return true
	}

	return errors.IsNotFound(err)
}

// IsQuotaExceeded checks if err is a googleapi error for exceeded quota.
func IsQuotaExceeded(err error) bool {
	if apiErr, ok := err.(*googleapi.Error); ok {
		for _, errItem := range apiErr.Errors {
			if errItem.Reason == quotaExceeded {
				return true
			}
		}
	}

	return false
}

// IgnoreNotFound returns nil if err is HTTP Not Found (404), else err.
func IgnoreNotFound(err error) error {
	if err == nil {
		return nil
	}

	if IsNotFound(err) {
		return nil
	}

	return err
}
