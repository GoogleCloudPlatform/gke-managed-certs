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

package marshaller

import (
	"testing"
)

func TestMarshal(t *testing.T) {
	m1 := map[string]string{"key1": "value2"}
	m2 := Unmarshal(Marshal(m1))

	v, e := m2["key1"]
	if !e || v != "value2" {
		t.Errorf("Marshalling and then unmarshalling mangles data: e is %t, want true; v: %s, want %s", e, v, m1["key1"])
	}
}
