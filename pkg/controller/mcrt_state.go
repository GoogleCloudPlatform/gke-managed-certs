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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"managed-certs-gke/pkg/client/configmap"
)

const (
	configMapName      = "managed-certificate-config"
	configMapNamespace = "kube-system"
	keySeparator       = ":"
)

type McrtState struct {
	sync.RWMutex

	// Maps Managed Certificate to SslCertificate name. Keys are built with buildKey() and decoded with splitKey().
	mapping map[string]string

	// Manages ConfigMap objects
	configmap configmap.Client
}

/*
* Transforms a namespace and name into a key in McrtState mapping.
 */
func buildKey(namespace, name string) string {
	return fmt.Sprintf("%s%s%s", namespace, keySeparator, name)
}

/*
* Transforms a key in McrtState mapping back into a namespace and name.
 */
func splitKey(key string) (string, string) {
	parts := strings.Split(key, keySeparator)
	return parts[0], parts[1]
}

func newMcrtState(configmap configmap.Client) *McrtState {
	mapping := make(map[string]string)

	if config, err := configmap.Get(configMapNamespace, configMapName); err != nil && len(config.Data) > 0 {
		mapping = unmarshal(config.Data)
	}

	return &McrtState{
		mapping:   mapping,
		configmap: configmap,
	}
}

func (state *McrtState) Delete(namespace, name string) {
	state.Lock()
	defer state.Unlock()
	delete(state.mapping, buildKey(namespace, name))
	state.persist()
}

func (state *McrtState) Get(namespace, name string) (string, bool) {
	state.RLock()
	defer state.RUnlock()
	value, exists := state.mapping[buildKey(namespace, name)]
	return value, exists
}

type Key struct {
	Namespace string
	Name      string
}

func (state *McrtState) GetAllKeys() []Key {
	var result []Key

	state.RLock()
	defer state.RUnlock()

	for key := range state.mapping {
		namespace, name := splitKey(key)
		result = append(result, Key{
			Namespace: namespace,
			Name:      name,
		})
	}

	return result
}

func (state *McrtState) Put(namespace, name, value string) {
	state.Lock()
	defer state.Unlock()

	state.mapping[buildKey(namespace, name)] = value
	state.persist()
}

func (state *McrtState) persist() {
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

/*
* A type used to marshal contents of McertState to json
 */
type configMapEntry struct {
	Key   string
	Value string
}

/*
* Transforms a given map into another map with keys formed of characters allowed by ConfigMap and values that encode entries of initial map.
 */
func marshal(m map[string]string) map[string]string {
	result := make(map[string]string)
	i := 0
	for k, v := range m {
		i++
		key := fmt.Sprintf("%d", i)
		value, _ := json.Marshal(configMapEntry{
			Key:   k,
			Value: v,
		})
		result[key] = string(value)
	}

	return result
}

/*
* Transforms an encoded map back into a map with contents of McertState.
 */
func unmarshal(m map[string]string) map[string]string {
	result := make(map[string]string)
	for _, v := range m {
		var entry configMapEntry
		_ = json.Unmarshal([]byte(v), &entry)
		result[entry.Key] = entry.Value
	}

	return result
}
