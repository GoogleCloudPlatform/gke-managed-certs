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

package controller

import (
	"sync"
)

type McertState struct {
	sync.RWMutex

	// Maps Managed Certificate name to SslCertificate resource name
	m map[string]string
}

func newMcertState() *McertState {
	return &McertState{
		m: make(map[string]string),
	}
}

func (state *McertState) Delete(key string) {
	state.Lock()
	defer state.Unlock()
	delete(state.m, key)
}

func (state *McertState) Get(key string) (string, bool) {
	state.RLock()
	defer state.RUnlock()
	value, exists := state.m[key]
	return value, exists
}

func (state *McertState) GetAllManagedCertificates() []string {
	var result []string

	state.RLock()
	defer state.RUnlock()

	for key := range state.m {
		result = append(result, key)
	}

	return result
}

func (state *McertState) GetAllSslCertificates() []string {
	var result []string

	state.RLock()
	defer state.RUnlock()

	for _, value := range state.m {
		result = append(result, value)
	}

	return result
}

func (state *McertState) Put(key, value string) {
	state.Lock()
	defer state.Unlock()

	state.m[key] = value
}
