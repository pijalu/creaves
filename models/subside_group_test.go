package models

import (
	"testing"
)

func TestSubsideGroupString(t *testing.T) {
	s := SubsideGroup{ID: "SG1", Group: "Large", Size: 5}
	if str := s.String(); str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestSubsideGroupsString(t *testing.T) {
	ss := SubsideGroups{{ID: "1"}, {ID: "2"}}
	if str := ss.String(); str == "" {
		t.Error("Expected non-empty string representation")
	}
}
