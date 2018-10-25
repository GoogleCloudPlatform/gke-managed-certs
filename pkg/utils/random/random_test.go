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

package random

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func newName(r Random, t *testing.T) string {
	if name, err := r.Name(); err != nil {
		t.Errorf("Failed to create random name: %v", err)
		return ""
	} else {
		return name
	}
}

func TestName_NonEmptyNameShorterThanLimit(t *testing.T) {
	sut := New()
	if name := newName(sut, t); len(name) <= 0 || len(name) >= 64 {
		t.Errorf("Name %s has %d characters, want between 0 and 63", name, len(name))
	}
}

func TestName_TwiceReturnsDifferent(t *testing.T) {
	sut := New()
	if name := newName(sut, t); name == newName(sut, t) {
		t.Errorf("Name called twice returned the same name %s, want different", name)
	}
}

type fakeReader struct {
}

var fakeReaderError = errors.New("fakeReader cannot read")

func (*fakeReader) Read(p []byte) (int, error) {
	return 0, fakeReaderError
}

func TestName_OnRandomNumberGeneratorError(t *testing.T) {
	uuid.SetRand(&fakeReader{})
	sut := New()
	name, err := sut.Name()

	if name != "" || err != fakeReaderError {
		t.Errorf("Name %s, want empty; error %v, want fakeReaderError %v", name, err, fakeReaderError)
	}
}
