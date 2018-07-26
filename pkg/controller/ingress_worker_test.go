package controller

import (
	"testing"
)

func TestParseAnnotation_empty(t *testing.T) {
	names := parseAnnotation("")

	if len(names) != 0 {
		t.Errorf("Empty value annotation \"\" should be parsed into empty array, is instead: %v.", names)
	}
}

func TestParseAnnotation_oneElement(t *testing.T) {
	names := parseAnnotation("xyz")

	if len(names) != 1 || names[0] != "xyz" {
		t.Errorf("One element array annotation \"xyz\" should be parsed into one element array with value xyz, is instead: %v.", names)
	}
}

func TestParseAnnotation_multiElement(t *testing.T) {
	names := parseAnnotation("xyz,abc")

	if len(names) != 2 || names[0] != "xyz" || names[1] != "abc" {
		t.Errorf("Multi element annotation \"xyz,abc\" should be parsed into two element array with values xyz, abc, is instead: %v.", names)
	}
}
