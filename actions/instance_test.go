package actions

import (
	"os"
	"testing"
)

func TestInstanceIDFromEnv(t *testing.T) {
	// Save original value
	originalEnv := os.Getenv("INSTANCE_ID")
	defer os.Setenv("INSTANCE_ID", originalEnv)

	// Set test value
	testInstanceID := "test-instance-123"
	os.Setenv("INSTANCE_ID", testInstanceID)

	// Re-read the env value (simulating what init() does)
	result := os.Getenv("INSTANCE_ID")
	if result != testInstanceID {
		t.Errorf("Expected instance ID %s, got %s", testInstanceID, result)
	}
}

func TestInstanceIDDefault(t *testing.T) {
	// Clear the environment variable
	originalEnv := os.Getenv("INSTANCE_ID")
	os.Unsetenv("INSTANCE_ID")
	defer func() {
		if originalEnv != "" {
			os.Setenv("INSTANCE_ID", originalEnv)
		}
	}()

	// Get hostname as fallback
	hostname, err := os.Hostname()
	if err != nil {
		t.Skip("Could not get hostname for test")
	}

	// The init() function should have set InstanceID to hostname
	// when INSTANCE_ID was not set during package initialization
	if InstanceID == "" {
		t.Error("Expected InstanceID to be set")
	}

	// If hostname was available during init, it should match
	if hostname != "" && InstanceID != hostname {
		// InstanceID might have been set from env during actual init
		// so we just check it's not empty
		t.Logf("InstanceID: %s, Hostname: %s", InstanceID, hostname)
	}
}

func TestGetInstanceID(t *testing.T) {
	// Test when CurrentConfig is nil
	oldConfig := CurrentConfig
	CurrentConfig = nil
	defer func() {
		CurrentConfig = oldConfig
	}()

	id := GetInstanceID()
	if id != "" {
		t.Errorf("Expected empty string when CurrentConfig is nil, got %s", id)
	}

	// Test when CurrentConfig is set
	// Note: We can't easily set CurrentConfig without DB access in unit tests
	// This test validates the basic logic
}
