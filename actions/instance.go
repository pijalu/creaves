package actions

import (
	"os"

	"github.com/gobuffalo/envy"
	"github.com/gofrs/uuid"
)

// InstanceID is the unique identifier for this application instance
// Used for multi-instance consolidation via event stream
// Deprecated: Use GetInstanceID() instead to get the value from database config
var InstanceID string

func init() {
	// Try to get instance ID from environment variable (fallback only)
	InstanceID = envy.Get("INSTANCE_ID", "")

	// If not set, try to use hostname
	if InstanceID == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			InstanceID = hostname
		} else {
			// Fallback to a generated UUID
			InstanceID = uuid.Must(uuid.NewV4()).String()
		}
	}
}
