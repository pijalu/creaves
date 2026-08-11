package models

import (
	"testing"

	"github.com/gofrs/uuid"
)

func TestConfigString(t *testing.T) {
	c := Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance-123",
		Name:       "Test Instance",
		Active:     true,
	}

	s := c.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConfigsString(t *testing.T) {
	cs := Configs{
		{
			ID:         uuid.Must(uuid.NewV4()),
			InstanceID: "test-instance-123",
			Name:       "Test Instance",
			Active:     true,
		},
	}

	s := cs.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConfigValidate(t *testing.T) {
	// Valid config
	c := Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test Instance",
	}

	verrs, err := c.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestConfigValidateMissingInstanceID(t *testing.T) {
	// Missing instance ID
	c := Config{
		ID:   uuid.Must(uuid.NewV4()),
		Name: "Test Instance",
	}

	verrs, err := c.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !verrs.HasAny() {
		t.Error("Expected validation errors for missing instance_id")
	}
}

func TestConfigValidateMissingName(t *testing.T) {
	// Missing name
	c := Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
	}

	verrs, err := c.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !verrs.HasAny() {
		t.Error("Expected validation errors for missing name")
	}
}

func TestConfigValidateCreate(t *testing.T) {
	c := Config{}

	verrs, err := c.ValidateCreate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestConfigValidateUpdate(t *testing.T) {
	c := Config{}

	verrs, err := c.ValidateUpdate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestConfigSettings(t *testing.T) {
	c := Config{}

	// Test default settings
	settings, err := c.GetSettings()
	if err != nil {
		t.Errorf("Expected no error getting default settings, got %v", err)
	}

	if !settings.EnableEventStream {
		t.Error("Expected EnableEventStream to be true by default")
	}

	// Test setting custom settings
	settings.EnableEventStream = false
	err = c.SetSettings(settings)
	if err != nil {
		t.Errorf("Expected no error setting settings, got %v", err)
	}

	// Verify settings were set
	newSettings, err := c.GetSettings()
	if err != nil {
		t.Errorf("Expected no error getting settings, got %v", err)
	}

	if newSettings.EnableEventStream {
		t.Error("Expected EnableEventStream to be false after update")
	}
}
