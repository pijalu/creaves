package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"os"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// CurrentConfig holds the cached configuration
var CurrentConfig *models.Config

// LoadConfig loads or creates the configuration from the database
func LoadConfig(tx *pop.Connection) (*models.Config, error) {
	// Try to find existing active config
	configs := &models.Configs{}
	err := tx.Where("active = ?", true).Order("created_at asc").Limit(1).All(configs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(*configs) > 0 {
		CurrentConfig = &(*configs)[0]
		return CurrentConfig, nil
	}

	// No config exists, create one
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			instanceID = hostname
		} else {
			instanceID = uuid.Must(uuid.NewV4()).String()
		}
	}

	config := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: instanceID,
		Name:       "Default Instance",
		Active:     true,
	}

	// Set default settings
	settings := models.DefaultSettings()
	if err := config.SetSettings(settings); err != nil {
		return nil, errors.WithStack(err)
	}

	verrs, err := tx.ValidateAndCreate(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if verrs.HasAny() {
		return nil, fmt.Errorf("validation errors: %v", verrs)
	}

	CurrentConfig = config
	return config, nil
}

// GetInstanceID returns the current instance ID from the loaded config
func GetInstanceID() string {
	if CurrentConfig != nil {
		return CurrentConfig.InstanceID
	}
	return ""
}

// IsEventStreamEnabled checks if the event stream is enabled in config
func IsEventStreamEnabled() bool {
	if CurrentConfig == nil {
		return false
	}
	settings, err := CurrentConfig.GetSettings()
	if err != nil {
		return false
	}
	return settings.EnableEventStream
}

// IsWebhookEnabled checks if the webhook is enabled in config
func IsWebhookEnabled() bool {
	if CurrentConfig == nil {
		return false
	}
	settings, err := CurrentConfig.GetSettings()
	if err != nil {
		return false
	}
	return settings.WebhookEnabled && settings.WebhookURL != ""
}

// ConfigsResource is the resource for the Config model
type ConfigsResource struct {
	buffalo.Resource
}

// requireAdmin checks if the current user is an admin
func requireAdmin(c buffalo.Context) (*models.User, error) {
	cu := GetCurrentUser(c)
	if cu == nil || !cu.Admin {
		return nil, c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required for this action"))
	}
	return cu, nil
}

// List gets all Configs. This function is mapped to the path
// GET /config
func (v ConfigsResource) List(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	configs := &models.Configs{}
	q := tx.PaginateFromParams(c.Params())

	if err := q.All(configs); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("pagination", q.Paginator)
		c.Set("configs", configs)
		return c.Render(http.StatusOK, r.HTML("config/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(configs))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(configs))
	}).Respond(c)
}

// Show gets the data for one Config. This function is mapped to
// the path GET /config/{config_id}
func (v ConfigsResource) Show(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	config := &models.Config{}
	if err := tx.Find(config, c.Param("config_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Parse settings for template display
	settings, _ := config.GetSettings()
	c.Set("settings", settings)

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("config", config)
		return c.Render(http.StatusOK, r.HTML("config/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(config))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(config))
	}).Respond(c)
}

// New renders the form for creating a new Config.
// This function is mapped to the path GET /config/new
func (v ConfigsResource) New(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	config := &models.Config{}
	// Set default settings
	settings := models.DefaultSettings()
	config.SetSettings(settings)
	
	c.Set("config", config)
	c.Set("settings", settings)
	return c.Render(http.StatusOK, r.HTML("config/new.plush.html"))
}

// Create adds a Config to the DB. This function is mapped to the
// path POST /config
func (v ConfigsResource) Create(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	config := &models.Config{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	config.ID = uuid.Must(uuid.NewV4())
	// Bind fields manually to avoid formam parsing issues
	config.InstanceID = c.Param("InstanceID")
	config.Name = c.Param("Name")
	config.Description = c.Param("Description")
	// Handle Active checkbox manually - it will be "true" if checked, missing if unchecked
	config.Active = c.Param("Active") == "true"

	// Build settings from form
	batchSize := 1
	maxPerMin := 60
	if bs := c.Param("Settings.WebhookBatchSize"); bs != "" {
		fmt.Sscanf(bs, "%d", &batchSize)
	}
	if mpm := c.Param("Settings.WebhookMaxPerMin"); mpm != "" {
		fmt.Sscanf(mpm, "%d", &maxPerMin)
	}

	settings := models.ConfigSettings{
		EnableEventStream: c.Param("Settings.EnableEventStream") == "true",
		WebhookEnabled:    c.Param("Settings.WebhookEnabled") == "true",
		WebhookURL:        c.Param("Settings.WebhookURL"),
		WebhookAPIKey:     c.Param("Settings.WebhookAPIKey"),
		WebhookBatchSize:  batchSize,
		WebhookMaxPerMin:  maxPerMin,
	}
	if err := config.SetSettings(settings); err != nil {
		return errors.WithStack(err)
	}

	verrs, err := tx.ValidateAndCreate(config)
	if err != nil {
		return errors.WithStack(err)
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			c.Set("errors", verrs)
			c.Set("config", config)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("config/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Config created successfully")
		return c.Redirect(http.StatusSeeOther, "/config/%v", config.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(config))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(config))
	}).Respond(c)
}

// Edit renders a edit form for a Config. This function is
// mapped to the path GET /config/{config_id}/edit
func (v ConfigsResource) Edit(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	config := &models.Config{}
	if err := tx.Find(config, c.Param("config_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Parse settings for template display
	settings, _ := config.GetSettings()
	c.Set("settings", settings)

	c.Set("config", config)
	return c.Render(http.StatusOK, r.HTML("config/edit.plush.html"))
}

// Update changes a Config in the DB. This function is mapped to
// the path PUT /config/{config_id}
func (v ConfigsResource) Update(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	config := &models.Config{}
	if err := tx.Find(config, c.Param("config_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind non-boolean fields manually to avoid formam parsing issues
	config.InstanceID = c.Param("InstanceID")
	config.Name = c.Param("Name")
	config.Description = c.Param("Description")
	// Handle Active checkbox manually - it will be "true" if checked, missing if unchecked
	config.Active = c.Param("Active") == "true"

	// Build settings from form
	batchSize := 1
	maxPerMin := 60
	if bs := c.Param("Settings.WebhookBatchSize"); bs != "" {
		fmt.Sscanf(bs, "%d", &batchSize)
	}
	if mpm := c.Param("Settings.WebhookMaxPerMin"); mpm != "" {
		fmt.Sscanf(mpm, "%d", &maxPerMin)
	}

	settings := models.ConfigSettings{
		EnableEventStream: c.Param("Settings.EnableEventStream") == "true",
		WebhookEnabled:    c.Param("Settings.WebhookEnabled") == "true",
		WebhookURL:        c.Param("Settings.WebhookURL"),
		WebhookAPIKey:     c.Param("Settings.WebhookAPIKey"),
		WebhookBatchSize:  batchSize,
		WebhookMaxPerMin:  maxPerMin,
	}
	if err := config.SetSettings(settings); err != nil {
		return errors.WithStack(err)
	}

	verrs, err := tx.ValidateAndUpdate(config)
	if err != nil {
		return errors.WithStack(err)
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			c.Set("errors", verrs)
			c.Set("config", config)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("config/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	// Update cached config if this is the current one
	if CurrentConfig != nil && CurrentConfig.ID == config.ID {
		CurrentConfig = config
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Config updated successfully")
		return c.Redirect(http.StatusSeeOther, "/config/%v", config.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(config))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(config))
	}).Respond(c)
}

// Destroy deletes a Config from the DB. This function is mapped
// to the path DELETE /config/{config_id}
func (v ConfigsResource) Destroy(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	config := &models.Config{}
	if err := tx.Find(config, c.Param("config_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Don't allow deleting the currently active config
	if CurrentConfig != nil && CurrentConfig.ID == config.ID {
		return c.Error(http.StatusBadRequest, fmt.Errorf("cannot delete the currently active configuration"))
	}

	if err := tx.Destroy(config); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Config deleted successfully")
		return c.Redirect(http.StatusSeeOther, "/config")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(config))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(config))
	}).Respond(c)
}
