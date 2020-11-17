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
	"context"
	"encoding/json"
	"fmt"
	"sync"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
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

type Interface interface {
	Delete(ctx context.Context, id types.Id)
	Get(id types.Id) (Entry, error)
	Insert(ctx context.Context, id types.Id, sslCertificateName string)
	List() map[types.Id]Entry
	SetExcludedFromSLO(ctx context.Context, id types.Id) error
	SetSoftDeleted(ctx context.Context, id types.Id) error
	SetSslCertificateBindingReported(ctx context.Context, id types.Id) error
	SetSslCertificateCreationReported(ctx context.Context, id types.Id) error
}

type impl struct {
	sync.RWMutex

	// Maps ManagedCertificate to SslCertificate
	mapping map[types.Id]Entry

	// Manages ConfigMap objects
	configmap configmap.Interface
}

func New(ctx context.Context, configmap configmap.Interface) Interface {
	mapping := make(map[types.Id]Entry)

	config, err := configmap.Get(ctx, configMapNamespace, configMapName)
	if err != nil {
		klog.Warning(err)
	} else if len(config.Data) > 0 {
		mapping = unmarshal(config.Data)
	}

	return &impl{
		mapping:   mapping,
		configmap: configmap,
	}
}

// Delete deletes entry associated with ManagedCertificate id.
func (state *impl) Delete(ctx context.Context, id types.Id) {
	state.Lock()
	defer state.Unlock()
	delete(state.mapping, id)
	state.persist(ctx)
}

// Get fetches an entry associated with ManagedCertificate id.
func (state *impl) Get(id types.Id) (Entry, error) {
	state.Lock()
	defer state.Unlock()

	entry, exists := state.mapping[id]
	if !exists {
		return Entry{}, errors.NotFound
	}

	return entry, nil
}

// Insert adds a new entry with an associated SslCertificate name.
// If an id already exists in state, it is overwritten.
func (state *impl) Insert(ctx context.Context, id types.Id, sslCertificateName string) {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		v = Entry{}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[id] = v
	state.persist(ctx)
}

// List fetches all data stored in state.
func (state *impl) List() map[types.Id]Entry {
	data := make(map[types.Id]Entry, 0)

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
func (state *impl) SetExcludedFromSLO(ctx context.Context, id types.Id) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.ExcludedFromSLO = true

	state.mapping[id] = v
	state.persist(ctx)

	return nil
}

// SetSoftDeleted sets to true a flag indicating that entry associated
// with given ManagedCertificate id has been deleted.
func (state *impl) SetSoftDeleted(ctx context.Context, id types.Id) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SoftDeleted = true

	state.mapping[id] = v
	state.persist(ctx)

	return nil
}

// SetSslCertificateBindingReported sets to true a flag indicating that
// SslCertificate binding metric has been already reported
// for this ManagedCertificate id.
func (state *impl) SetSslCertificateBindingReported(ctx context.Context, id types.Id) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SslCertificateBindingReported = true

	state.mapping[id] = v
	state.persist(ctx)

	return nil
}

// SetSslCertificateCreationReported sets to true a flag indicating that
// SslCertificate creation metric has been already reported
// for this ManagedCertificate id.
func (state *impl) SetSslCertificateCreationReported(ctx context.Context, id types.Id) error {
	state.Lock()
	defer state.Unlock()

	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SslCertificateCreationReported = true

	state.mapping[id] = v
	state.persist(ctx)

	return nil
}

func (state *impl) persist(ctx context.Context) {
	config := &api.ConfigMap{
		Data: marshal(state.mapping),
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
	}
	if err := state.configmap.UpdateOrCreate(ctx, configMapNamespace, config); err != nil {
		runtime.HandleError(err)
	}
}

// jsonMapEntry stores an entry in a map being marshalled to JSON.
type jsonMapEntry struct {
	Key   types.Id
	Value Entry
}

// Transforms input map m into a new map which can be stored in a ConfigMap.
// Values in new map encode entries of m.
func marshal(m map[types.Id]Entry) map[string]string {
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
func unmarshal(m map[string]string) map[types.Id]Entry {
	result := make(map[types.Id]Entry)
	for _, v := range m {
		var entry jsonMapEntry
		_ = json.Unmarshal([]byte(v), &entry)
		result[entry.Key] = entry.Value
	}

	return result
}
