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

package utils

import (
	"errors"
	"time"

	"k8s.io/klog"
)

const (
	maxRetries = 80
	timeout    = 30
)

var retryError = errors.New("Retry failed")

func Retry(action func() error) error {
	for i := 1; i <= maxRetries; i++ {
		err := action()
		if err == nil {
			return nil
		}

		if i < maxRetries {
			klog.Warningf("%d. retry in %d seconds because of %s", i, timeout, err.Error())
			time.Sleep(timeout * time.Second)
		}
	}

	klog.Errorf("Failed because of exceeding the retry limit")
	return retryError
}
