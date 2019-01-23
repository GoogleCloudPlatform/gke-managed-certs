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

// Package stage stores controller state and persists it in a ConfigMap.
package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/golang/glog"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	configMapName      = "managed-certificate-config"
	configMapNamespace = "kube-system"
)

type StateIterator interface {
	ForeachKey(f func(id types.CertId))
}

type State interface {
	StateIterator

	Delete(id types.CertId)
	GetSslCertificateName(id types.CertId) (string, error)
	IsSoftDeleted(id types.CertId) (bool, error)
	IsSslCertificateCreationReported(id types.CertId) (bool, error)
	SetSoftDeleted(id types.CertId) error
	SetSslCertificateCreationReported(id types.CertId) error
	SetSslCertificateName(id types.CertId, sslCertificateName string)
}

type entry struct {
	SoftDeleted                    bool
	SslCertificateName             string
	SslCertificateCreationReported bool
}

type stateImpl struct {
	sync.RWMutex

	// Maps ManagedCertificate to SslCertificate
	mapping map[types.CertId]entry

	// Manages ConfigMap objects
	configmap configmap.ConfigMap
}

func New(configmap configmap.ConfigMap) State {
	mapping := make(map[types.CertId]entry)

	config, err := configmap.Get(configMapNamespace, configMapName)
	if err != nil {
		glog.Warning(err)
	} else if len(config.Data) > 0 {
		mapping = unmarshal(config.Data)
	}

	return &stateImpl{
		mapping:   mapping,
		configmap: configmap,
	}
}

// Delete deletes entry associated with ManagedCertificate id
func (state *stateImpl) Delete(id types.CertId) {
	state.Lock()
	defer state.Unlock()
	delete(state.mapping, id)
	state.persist()
}

// ForeachKey calls f on every id
func (state *stateImpl) ForeachKey(f func(id types.CertId)) {
	var ids []types.CertId

	state.RLock()
	for id := range state.mapping {
		ids = append(ids, id)
	}
	state.RUnlock()

	for _, id := range ids {
		f(id)
	}
}

// GetSslCertificateName returns the name of SslCertificate associated with ManagedCertificate id
func (state *stateImpl) GetSslCertificateName(id types.CertId) (string, error) {
	state.RLock()
	defer state.RUnlock()
	entry, exists := state.mapping[id]

	if !exists {
		return "", errors.ErrManagedCertificateNotFound
	}

	return entry.SslCertificateName, nil
}

// IsSoftDeleted returns true if entry associated with given ManagedCertificate id is marked as soft deleted.
func (state *stateImpl) IsSoftDeleted(id types.CertId) (bool, error) {
	state.RLock()
	defer state.RUnlock()
	entry, exists := state.mapping[id]

	if !exists {
		return false, errors.ErrManagedCertificateNotFound
	}

	return entry.SoftDeleted, nil
}

// IsSslCertificateCreationReported returns true if SslCertificate creation metric has already been reported for ManagedCertificate id
func (state *stateImpl) IsSslCertificateCreationReported(id types.CertId) (bool, error) {
	state.RLock()
	defer state.RUnlock()
	entry, exists := state.mapping[id]

	if !exists {
		return false, errors.ErrManagedCertificateNotFound
	}

	return entry.SslCertificateCreationReported, nil
}

// SetSoftDeleted sets to true a flag indicating that entry associated with given ManagedCertificate id has been deleted.
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

// SetSslCertificateCreationReported sets to true a flag indicating that SslCertificate creation metric has been already reported for ManagedCertificate id
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

// SetSslCertificateName sets the name of SslCertificate associated with ManagedCertificate id
func (state *stateImpl) SetSslCertificateName(id types.CertId, sslCertificateName string) {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		v = entry{}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[id] = v
	state.persist()
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
	Value entry
}

// Transforms input map m into a new map which can be stored in a ConfigMap. Values in new map encode entries of m.
func marshal(m map[types.CertId]entry) map[string]string {
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
func unmarshal(m map[string]string) map[types.CertId]entry {
	result := make(map[types.CertId]entry)
	for _, v := range m {
		var entry jsonMapEntry
		_ = json.Unmarshal([]byte(v), &entry)
		result[entry.Key] = entry.Value
	}

	return result
}
