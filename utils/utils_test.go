package utils

import (
	"testing"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/assert"
)

// trimFieldsSample is a struct exercising the string/nulls.String/non-string paths.
type trimFieldsSample struct {
	Name    string
	Label   string
	Count   int
	Notes   nulls.String
	Tag     nulls.String
	Private string // unexported: not settable via reflection
}

func TestTrimStringFields_TrimsStringFields(t *testing.T) {
	s := &trimFieldsSample{
		Name:  "  alice   ",
		Label: "hello world   ",
		Count: 42,
	}
	TrimStringFields(s)

	// Only trailing spaces are trimmed (TrimRight); leading spaces remain by design.
	assert.Equal(t, "  alice", s.Name)
	assert.Equal(t, "hello world", s.Label)
}

func TestTrimStringFields_TrimsNullsStringFields(t *testing.T) {
	s := &trimFieldsSample{
		Notes: nulls.NewString("  padded   "),
		Tag:   nulls.NewString("nopad"),
	}
	TrimStringFields(s)

	// Only trailing spaces trimmed; leading spaces remain.
	assert.True(t, s.Notes.Valid)
	assert.Equal(t, "  padded", s.Notes.String)

	assert.True(t, s.Tag.Valid)
	assert.Equal(t, "nopad", s.Tag.String)
}

func TestTrimStringFields_InvalidNullsStringUntouched(t *testing.T) {
	// A nulls.String that is not Valid should be left untouched.
	s := &trimFieldsSample{
		Tag: nulls.String{Valid: false},
	}
	TrimStringFields(s)

	assert.False(t, s.Tag.Valid)
	assert.Equal(t, "", s.Tag.String)
}

func TestTrimStringFields_NonStringFieldsUntouched(t *testing.T) {
	s := &trimFieldsSample{
		Name:  "trim me   ",
		Count: 7,
	}
	TrimStringFields(s)

	// Non-string numeric field must remain unchanged.
	assert.Equal(t, 7, s.Count)
	assert.Equal(t, "trim me", s.Name)
}

func TestTrimStringFields_NonPointerInput(t *testing.T) {
	// Passing a non-pointer value must not panic; it returns early.
	assert.NotPanics(t, func() {
		TrimStringFields(trimFieldsSample{Name: "  no panic   "})
	})
}

func TestTrimStringFields_NonStructPointer(t *testing.T) {
	// A pointer to a non-struct must not panic either.
	str := "  hello   "
	assert.NotPanics(t, func() {
		TrimStringFields(&str)
	})
	// String value untouched since target is not a struct.
	assert.Equal(t, "  hello   ", str)
}

func TestTrimStringFields_StructWithoutStringFields(t *testing.T) {
	type onlyNumbers struct {
		A int
		B float64
	}
	s := &onlyNumbers{A: 3, B: 1.5}
	assert.NotPanics(t, func() {
		TrimStringFields(s)
	})
	assert.Equal(t, 3, s.A)
	assert.Equal(t, 1.5, s.B)
}

func TestTrimStringFields_NilPointer(t *testing.T) {
	// A nil interface must not panic.
	assert.NotPanics(t, func() {
		TrimStringFields(nil)
	})
}
