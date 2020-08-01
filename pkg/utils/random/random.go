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

// Package random contains utilities for generating random names.
package random

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	maxNameLength = 63
)

type Interface interface {
	Name() (string, error)
}

type impl struct {
	sslCertificateNamePrefix string
}

func New(sslCertificateNamePrefix string) Interface {
	return impl{sslCertificateNamePrefix: sslCertificateNamePrefix}
}

// Name generates a random name for SslCertificate resource.
func (r impl) Name() (string, error) {
	if uid, err := uuid.NewRandom(); err != nil {
		return "", err
	} else {
		generatedName := fmt.Sprintf("%s%s", r.sslCertificateNamePrefix, uid.String())
		maxLength := maxNameLength
		if len(generatedName) < maxLength {
			maxLength = len(generatedName)
		}

		return generatedName[:maxLength], nil
	}
}
