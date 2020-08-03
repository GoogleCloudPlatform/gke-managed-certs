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

// Package types defines helpers for manipulating types.
package types

import (
	"fmt"
)

const (
	separator = ":"
)

// Id identifies a Kubernetes resource within a cluster.
type Id struct {
	Namespace string
	Name      string
}

func NewId(namespace, name string) Id {
	return Id{
		Namespace: namespace,
		Name:      name,
	}
}

// String converts Id to string.
func (id Id) String() string {
	return fmt.Sprintf("%s%s%s", id.Namespace, separator, id.Name)
}
