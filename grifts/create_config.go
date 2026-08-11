package grifts

import (
	"creaves/actions"
	"creaves/models"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

func createConfig(c *grift.Context) error {
	return models.DB.Transaction(func(tx *pop.Connection) error {
		// Check if config already exists
		exists, err := tx.Where("active = ?", true).Exists(&models.Config{})
		if err != nil {
			return err
		}
		if exists {
			return nil
		}

		// Create default config with settings
		config := &models.Config{
			ID:         uuid.Must(uuid.NewV4()),
			InstanceID: actions.InstanceID,
			Name:       "Default Instance",
			Active:     true,
		}

		// Set default settings with feature flags
		settings := models.DefaultSettings()
		if err := config.SetSettings(settings); err != nil {
			return err
		}

		return tx.Create(config)
	})
}
