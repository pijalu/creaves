package models

import (
	"testing"
)

func TestNativeStatusString(t *testing.T) {
	n := NativeStatus{ID: "NS1", Status: "Native"}
	if s := n.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestNativeStatusesString(t *testing.T) {
	ns := NativeStatuses{{ID: "1"}, {ID: "2"}}
	if s := ns.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
