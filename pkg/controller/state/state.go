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

// Package stage stores controller state and persists it in a ConfigMap.
package state

import (
	"encoding/json"
	"fmt"
	"sync"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	configMapName      = "managed-certificate-config"
	configMapNamespace = "kube-system"
)

type Entry struct {
	ExcludedFromSLO                bool
	SoftDeleted                    bool
	SslCertificateName             string
	SslCertificateBindingReported  bool
	SslCertificateCreationReported bool
}

type State interface {
	Delete(id types.CertId)
	Get(id types.CertId) (Entry, error)
	Insert(id types.CertId, sslCertificateName string)
	List() map[types.CertId]Entry
	SetExcludedFromSLO(id types.CertId) error
	SetSoftDeleted(id types.CertId) error
	SetSslCertificateBindingReported(id types.CertId) error
	SetSslCertificateCreationReported(id types.CertId) error
}

type stateImpl struct {
	sync.RWMutex

	// Maps ManagedCertificate to SslCertificate
	mapping map[types.CertId]Entry

	// Manages ConfigMap objects
	configmap configmap.ConfigMap
}

func New(configmap configmap.ConfigMap) State {
	mapping := make(map[types.CertId]Entry)

	config, err := configmap.Get(configMapNamespace, configMapName)
	if err != nil {
		klog.Warning(err)
	} else if len(config.Data) > 0 {
		mapping = unmarshal(config.Data)
	}

	return &stateImpl{
		mapping:   mapping,
		configmap: configmap,
	}
}

// Delete deletes entry associated with ManagedCertificate id.
func (state *stateImpl) Delete(id types.CertId) {
	state.Lock()
	defer state.Unlock()
	delete(state.mapping, id)
	state.persist()
}

// Get fetches an entry associated with ManagedCertificate id.
func (state *stateImpl) Get(id types.CertId) (Entry, error) {
	state.Lock()
	defer state.Unlock()

	entry, exists := state.mapping[id]
	if !exists {
		return Entry{}, errors.ErrManagedCertificateNotFound
	}

	return entry, nil
}

// Insert adds a new entry with an associated SslCertificate name.
// If an id already exists in state, it is overwritten.
func (state *stateImpl) Insert(id types.CertId, sslCertificateName string) {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		v = Entry{}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[id] = v
	state.persist()
}

// List fetches all data stored in state.
func (state *stateImpl) List() map[types.CertId]Entry {
	data := make(map[types.CertId]Entry, 0)

	state.RLock()
	defer state.RUnlock()
	for id, entry := range state.mapping {
		data[id] = entry
	}

	return data
}

// SetExcludedFromSLO sets to true a flag indicating that entry associated
// with given ManagedCertificate id should not be taken into account
// for the purposes of SLO calculation.
func (state *stateImpl) SetExcludedFromSLO(id types.CertId) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.ExcludedFromSLO = true

	state.mapping[id] = v
	state.persist()

	return nil
}

// SetSoftDeleted sets to true a flag indicating that entry associated
// with given ManagedCertificate id has been deleted.
func (state *stateImpl) SetSoftDeleted(id types.CertId) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SoftDeleted = true

	state.mapping[id] = v
	state.persist()

	return nil
}

// SetSslCertificateBindingReported sets to true a flag indicating that
// SslCertificate binding metric has been already reported
// for this ManagedCertificate id.
func (state *stateImpl) SetSslCertificateBindingReported(id types.CertId) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SslCertificateBindingReported = true

	state.mapping[id] = v
	state.persist()

	return nil
}

// SetSslCertificateCreationReported sets to true a flag indicating that
// SslCertificate creation metric has been already reported
// for this ManagedCertificate id.
func (state *stateImpl) SetSslCertificateCreationReported(id types.CertId) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SslCertificateCreationReported = true

	state.mapping[id] = v
	state.persist()

	return nil
}

func (state *stateImpl) persist() {
	config := &api.ConfigMap{
		Data: marshal(state.mapping),
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
	}
	if err := state.configmap.UpdateOrCreate(configMapNamespace, config); err != nil {
		runtime.HandleError(err)
	}
}

// jsonMapEntry stores an entry in a map being marshalled to JSON.
type jsonMapEntry struct {
	Key   types.CertId
	Value Entry
}

// Transforms input map m into a new map which can be stored in a ConfigMap.
// Values in new map encode entries of m.
func marshal(m map[types.CertId]Entry) map[string]string {
	result := make(map[string]string)
	i := 0
	for k, v := range m {
		i++
		key := fmt.Sprintf("%d", i)
		value, _ := json.Marshal(jsonMapEntry{
			Key:   k,
			Value: v,
		})
		result[key] = string(value)
	}

	return result
}

// Transforms an encoded map back into initial map.
func unmarshal(m map[string]string) map[types.CertId]Entry {
	result := make(map[types.CertId]Entry)
	for _, v := range m {
		var entry jsonMapEntry
		_ = json.Unmarshal([]byte(v), &entry)
		result[entry.Key] = entry.Value
	}

	return result
}
