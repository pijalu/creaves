package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"os"
	"strings"

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

// EnableWebhookForwarding turns webhook forwarding on for the active config
// (confirm-to-enable path from the resync page). It requires a webhook URL
// to already be configured — enabling delivery with no destination would
// silently queue events forever. Persists on tx, refreshes CurrentConfig,
// and ensures the delivery/purge worker is running.
func EnableWebhookForwarding(tx *pop.Connection) error {
	if CurrentConfig == nil {
		if _, err := LoadConfig(tx); err != nil {
			return fmt.Errorf("no active config: configure the webhook URL first")
		}
	}
	settings, err := CurrentConfig.GetSettings()
	if err != nil {
		return err
	}
	if strings.TrimSpace(settings.WebhookURL) == "" {
		return fmt.Errorf("no webhook URL configured: set the console URL before enabling")
	}
	settings.WebhookEnabled = true
	if err := CurrentConfig.SetSettings(settings); err != nil {
		return err
	}
	// Persist on models.DB (autocommit) rather than the request tx: the
	// buffalo pop transaction middleware rolls back the request tx after a
	// redirect response, which would silently discard the flag flip.
	persistTx := models.DB
	if persistTx == nil {
		persistTx = tx
	}
	verrs, err := persistTx.ValidateAndUpdate(CurrentConfig)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("config validation failed: %v", verrs)
	}
	EnsureWebhookWorkerRunning()
	return nil
}

// ConfigsResource is the resource for the Config model
type ConfigsResource struct {
	buffalo.Resource
}

// parseWebhookLimits parses and clamps the webhook delivery limits from form
// parameters into their documented ranges (batch 1-100, rate 1-10000), so an
// invalid value can never reach the delivery worker (SQL LIMIT, request size).
func parseWebhookLimits(c buffalo.Context) (batchSize, maxPerMin int) {
	batchSize = 1
	maxPerMin = 60
	if bs := c.Param("Settings.WebhookBatchSize"); bs != "" {
		fmt.Sscanf(bs, "%d", &batchSize)
	}
	if mpm := c.Param("Settings.WebhookMaxPerMin"); mpm != "" {
		fmt.Sscanf(mpm, "%d", &maxPerMin)
	}
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 100 {
		batchSize = 100
	}
	if maxPerMin < 1 {
		maxPerMin = 1
	}
	if maxPerMin > 10000 {
		maxPerMin = 10000
	}
	return batchSize, maxPerMin
}

// paramIsTrue reports whether a boolean form field was checked.
// Checkbox fields are rendered with a hidden "false" input alongside the
// checkbox "true" input (see templates/config/_form.plush*.html), so the
// submitted key can carry two values. c.Param returns only the first value
// (hidden "false" comes first in the form), which would always read as
// unchecked. Inspecting every submitted value fixes that.
func paramIsTrue(c buffalo.Context, key string) bool {
	req := c.Request()
	if req == nil {
		return c.Param(key) == "true"
	}
	if req.Form != nil {
		for _, v := range req.Form[key] {
			if v == "true" {
				return true
			}
		}
		if _, present := req.Form[key]; present {
			return false
		}
	}
	return c.Param(key) == "true"
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
		// Expose the active config ID so the template can hide the delete
		// button for the configuration that cannot be deleted.
		currentConfigID := ""
		if CurrentConfig != nil {
			currentConfigID = CurrentConfig.ID.String()
		}
		c.Set("currentConfigID", currentConfigID)
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
	config.Active = paramIsTrue(c, "Active")

	// Build settings from form
	batchSize, maxPerMin := parseWebhookLimits(c)

	settings := models.ConfigSettings{
		EnableEventStream: paramIsTrue(c, "Settings.EnableEventStream"),
		WebhookEnabled:    paramIsTrue(c, "Settings.WebhookEnabled"),
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
	config.Active = paramIsTrue(c, "Active")

	// Build settings from form
	batchSize, maxPerMin := parseWebhookLimits(c)

	settings := models.ConfigSettings{
		EnableEventStream: paramIsTrue(c, "Settings.EnableEventStream"),
		WebhookEnabled:    paramIsTrue(c, "Settings.WebhookEnabled"),
		WebhookURL:        c.Param("Settings.WebhookURL"),
		WebhookAPIKey:     c.Param("Settings.WebhookAPIKey"),
		WebhookBatchSize:  batchSize,
		WebhookMaxPerMin:  maxPerMin,
	}
	// The API key field is intentionally NOT pre-filled in the form (to avoid
	// exposing the secret in the HTML). If the admin left it blank, preserve
	// the previously stored key rather than wiping it.
	if settings.WebhookAPIKey == "" {
		if existing, err := config.GetSettings(); err == nil {
			settings.WebhookAPIKey = existing.WebhookAPIKey
		}
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

	// If webhook forwarding was just enabled, start the delivery worker now.
	EnsureWebhookWorkerRunning()

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
