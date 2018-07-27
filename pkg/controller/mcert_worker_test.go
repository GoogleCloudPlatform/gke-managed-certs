package controller

import (
	"testing"
)

func TestCreateRandomName_returnsNonEmptyNameShorterThan64Characters(t *testing.T) {
	name, err := createRandomName()

	if err != nil {
		t.Errorf("Failed to create random name: %v", err)
	}

	if len(name) <= 0 || len(name) >= 64 {
		t.Errorf("Random name %v has %v characters, should have between 0 and 63", name, len(name))
	}
}

func TestCreateRandomName_calledTwiceReturnsDifferentNames(t *testing.T) {
	name1, err := createRandomName()

	if err != nil {
		t.Errorf("Failed to create random name1: %v", err)
	}

	name2, err := createRandomName()
	if err != nil {
		t.Errorf("Failed to create random name2 %v", err)
	}

	if name1 == name2 {
		t.Errorf("createRandomName called twice returned the same name %v", name1)
	}
}
