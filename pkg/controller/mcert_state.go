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

// SslCertificateState contains: Current, the name of associated SslCertificate and New, the name of a new SslCertificate if update is in progress
type SslCertificateState struct {
	Current string
	New     string
}

type McertState struct {
	sync.RWMutex

	// Maps Managed Certificate name to SslCertificateState
	m map[string]SslCertificateState
}

func newMcertState() *McertState {
	return &McertState{
		m: make(map[string]SslCertificateState),
	}
}

func (state *McertState) Delete(key string) {
	state.Lock()
	defer state.Unlock()
	delete(state.m, key)
}

func (state *McertState) Get(key string) (SslCertificateState, bool) {
	state.RLock()
	defer state.RUnlock()
	value, exists := state.m[key]
	return value, exists
}

func (state *McertState) GetAllManagedCertificates() (values []string) {
	var values []string
	// [review] values = make([]string, 0)

	state.RLock()
	defer state.RUnlock()

	for key := range state.m {
		values = append(values, key)
	}

	return
}

func (state *McertState) GetAllSslCertificates() (values []string) {
	var values []string
	// [review] values = make([]string, 0)

	state.RLock()
	defer state.RUnlock()

	for _, value := range state.m {
		values = append(values, value.Current)

		if value.New != "" {
			values = append(values, value.New)
		}
	}

	return
}

func (state *McertState) PutCurrent(key, value string) {
	state.Lock()
	defer state.Unlock()

	state.m[key] = SslCertificateState{
		Current: value,
		New:     "",
	}
}

func (state *McertState) PutState(key string, sslState SslCertificateState) { // Put(...), State is redundant.
	state.Lock()
	defer state.Unlock()
	state.m[key] = sslState
}
