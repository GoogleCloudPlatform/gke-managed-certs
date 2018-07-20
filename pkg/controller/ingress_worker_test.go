package controller

import (
	"testing"
)

func TestParseAnnotation_emptyString(t *testing.T) {
	_, err := parseAnnotation("")
	if err == nil {
		t.Errorf("Empty string annotation \"\" should not pass parsing.")
	}
}

func TestParseAnnotation_emptyArray(t *testing.T) {
	names, err := parseAnnotation("[]")
	if err != nil {
		t.Errorf("Empty array annotation [] should pass parsing, err: %v.", err)
	}
	if len(names) != 0 {
		t.Errorf("Empty array annotation [] should be parsed into empty array, is instead: %v.", names)
	}
}

func TestParseAnnotation_oneElementArray(t *testing.T) {
	names, err := parseAnnotation("[\"xyz\"]")
	if err != nil {
		t.Errorf("One element array annotation [\"xyz\"] should pass parsing, err: %v.", err)
	}
	if len(names) != 1 || names[0] != "xyz" {
		t.Errorf("One element array annotation [\"xyz\"] should be parsed into one element array with value xyz, is instead: %v.", names)
	}
}

func TestParseAnnotation_multiElementArray(t *testing.T) {
	names, err := parseAnnotation("[\"xyz\", \"abc\"]")
	if err != nil {
		t.Errorf("Multi element array annotation [\"xyz\", \"abc\"] should pass parsing, err: %v.", err)
	}
	if len(names) != 2 || names[0] != "xyz" || names[1] != "abc" {
		t.Errorf("Multi element array annotation [\"xyz\", \"abc\"] should be parsed into two element array with values xyz, abc, is instead: %v.", names)
	}
}
