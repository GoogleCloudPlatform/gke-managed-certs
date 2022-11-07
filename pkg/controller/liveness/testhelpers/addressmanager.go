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

// Package testhelpers defines helpers for liveness package.
package testhelpers

import (
	"fmt"
	"sync"
)

// AddressManager manages http server addresses to allow running multiple
// http servers in parallel without conflicting addresses. It does that
// by changing the port after returning an address.
type AddressManager struct {
	mutex sync.Mutex
	port  int
}

// NewAddressManager creates and returns an AddressManager with the
// starting port number 27100
func NewAddressManager() AddressManager {
	return AddressManager{port: 27100}
}

// getNextAddress creates an http server address in the form ":[healthCheckServerPort]"
// and returns it. It also increments healthCheckServerPort.
func (a *AddressManager) GetNextAddress() string {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	address := fmt.Sprintf(":%d", a.port)
	a.port++
	return address
}
