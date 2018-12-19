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
	"strings"
	"sync"

	"github.com/golang/glog"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
)

const (
	configMapName      = "managed-certificate-config"
	configMapNamespace = "kube-system"
	keySeparator       = ":"
)

type State interface {
	Delete(namespace, name string)
	ForeachKey(f func(namespace, name string))
	GetSslCertificateName(namespace, name string) (string, bool)
	IsSslCertificateCreationReported(namespace, name string) (bool, bool)
	SetSslCertificateCreationReported(namespace, name string)
	SetSslCertificateName(namespace, name, sslCertificateName string)
}

type entry struct {
	SslCertificateName             string
	SslCertificateCreationReported bool
}

type stateImpl struct {
	sync.RWMutex

	// Maps ManagedCertificate to SslCertificate. Keys are built with buildKey() and decoded with splitKey().
	mapping map[string]entry

	// Manages ConfigMap objects
	configmap configmap.ConfigMap
}

// Transforms a ManagedCertificate namespace and name into a key in State mapping.
func buildKey(namespace, name string) string {
	return fmt.Sprintf("%s%s%s", namespace, keySeparator, name)
}

// Transforms a key in State mapping back into a ManagedCertificate namespace and name.
func splitKey(key string) (string, string) {
	parts := strings.Split(key, keySeparator)
	return parts[0], parts[1]
}

func New(configmap configmap.ConfigMap) State {
	mapping := make(map[string]entry)

	config, err := configmap.Get(configMapNamespace, configMapName)
	if err != nil {
		glog.Warning(err)
	} else if len(config.Data) > 0 {
		mapping = unmarshal(config.Data)
	}

	return stateImpl{
		mapping:   mapping,
		configmap: configmap,
	}
}

// Delete deletes entry associated with ManagedCertificate identified by namespace and name
func (state stateImpl) Delete(namespace, name string) {
	state.Lock()
	defer state.Unlock()
	delete(state.mapping, buildKey(namespace, name))
	state.persist()
}

// ForeachKey calls f on every key split into namespace and name
func (state stateImpl) ForeachKey(f func(namespace, name string)) {
	var keys []string

	state.RLock()
	for k := range state.mapping {
		keys = append(keys, k)
	}
	state.RUnlock()

	for _, k := range keys {
		namespace, name := splitKey(k)
		f(namespace, name)
	}
}

// GetSslCertificateName returns the name of SslCertificate associated with ManagedCertificate identified by namespace and name
func (state stateImpl) GetSslCertificateName(namespace, name string) (string, bool) {
	state.RLock()
	defer state.RUnlock()
	entry, exists := state.mapping[buildKey(namespace, name)]
	return entry.SslCertificateName, exists
}

// IsSslCertificateCreationReported returns true if SslCertificate creation metric has already been reported for ManagedCertificate identified by namespace and name
func (state stateImpl) IsSslCertificateCreationReported(namespace, name string) (bool, bool) {
	state.RLock()
	defer state.RUnlock()
	entry, exists := state.mapping[buildKey(namespace, name)]
	return entry.SslCertificateCreationReported, exists
}

// SetSslCertificateCreationReported sets to true a flag indicating that SslCertificate creation metric has been already reported for ManagedCertificate identified by namespace and name
func (state stateImpl) SetSslCertificateCreationReported(namespace, name string) {
	state.Lock()
	defer state.Unlock()

	key := buildKey(namespace, name)
	v, exists := state.mapping[key]
	if !exists {
		v = entry{SslCertificateName: ""}
	}

	v.SslCertificateCreationReported = true

	state.mapping[key] = v
	state.persist()
}

// SetSslCertificateName sets the name of SslCertificate associated with ManagedCertificate identified by namespace and name
func (state stateImpl) SetSslCertificateName(namespace, name, sslCertificateName string) {
	state.Lock()
	defer state.Unlock()

	key := buildKey(namespace, name)
	v, exists := state.mapping[key]
	if !exists {
		v = entry{SslCertificateCreationReported: false}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[key] = v
	state.persist()
}

func (state stateImpl) persist() {
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
	Key   string
	Value entry
}

// Transforms input map m into a new map which can be stored in a ConfigMap. Values in new map encode entries of m.
func marshal(m map[string]entry) map[string]string {
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
func unmarshal(m map[string]string) map[string]entry {
	result := make(map[string]entry)
	for _, v := range m {
		var entry jsonMapEntry
		_ = json.Unmarshal([]byte(v), &entry)
		result[entry.Key] = entry.Value
	}

	return result
}
