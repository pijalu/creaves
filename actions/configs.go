package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// configMu guards CurrentConfig: the webhook delivery worker goroutine reads
// the cached config concurrently with request-time reloads (config CRUD,
// confirm-to-enable) and test resets.
var configMu sync.RWMutex

// CurrentConfig holds the cached configuration. Read it through
// CurrentConfigGet() and replace it through CurrentConfigSet() — direct
// access races with the webhook worker goroutine.
var CurrentConfig *models.Config

// CurrentConfigGet returns the cached config (nil when not yet loaded).
func CurrentConfigGet() *models.Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return CurrentConfig
}

// CurrentConfigSet replaces the cached config (nil forces the next LoadConfig
// to reload from the database). Stopping the webhook worker here guarantees a
// stale worker goroutine can never read the old config while callers replace
// it; flows that need delivery call EnsureWebhookWorkerRunning afterwards
// (config Update, EnableWebhookForwarding) or restart it lazily on the next
// publish.
func CurrentConfigSet(c *models.Config) {
	configMu.Lock()
	CurrentConfig = c
	configMu.Unlock()
	StopWebhookWorker()
}

// LoadConfig loads or creates the configuration from the database.
//
// Single-init guard: CurrentConfig is the process-local singleton, so once it
// is loaded subsequent calls return it without re-querying the config table
// (this runs on hot paths like SetCurrentUser and event production). Mutating
// flows keep the pointer fresh themselves — ConfigsResource Update/Activate
// and EnableWebhookForwarding assign or update CurrentConfig explicitly.
// Tests reset CurrentConfig to nil to force a reload.
func LoadConfig(tx *pop.Connection) (*models.Config, error) {
	if CurrentConfigGet() != nil {
		return CurrentConfigGet(), nil
	}

	// Try to find existing active config
	configs := &models.Configs{}
	err := tx.Where("active = ?", true).Order("created_at asc").Limit(1).All(configs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(*configs) > 0 {
		CurrentConfigSet(&(*configs)[0])
		return CurrentConfigGet(), nil
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

	CurrentConfigSet(config)
	return config, nil
}

// GetInstanceID returns the current instance ID from the loaded config
func GetInstanceID() string {
	if CurrentConfigGet() != nil {
		return CurrentConfigGet().InstanceID
	}
	return ""
}

// IsEventStreamEnabled checks if the event stream is enabled in config
func IsEventStreamEnabled() bool {
	if CurrentConfigGet() == nil {
		return false
	}
	settings, err := CurrentConfigGet().GetSettings()
	if err != nil {
		return false
	}
	return settings.EnableEventStream
}

// IsWebhookEnabled reports whether webhook forwarding can deliver: at
// least one sync target is enabled and configured with a URL. DB-backed —
// the flag no longer lives on the config settings.
func IsWebhookEnabled() bool {
	db := models.DB
	if db == nil {
		return false
	}
	ok, err := models.HasEnabledSyncTarget(db)
	if err != nil {
		return false
	}
	return ok
}

// EnableWebhookForwarding enables every sync target that has a webhook URL
// configured (confirm-to-enable path from the resync page). It requires at
// least one such target — enabling delivery with no destination would
// silently queue events forever. Persists on models.DB (autocommit) rather
// than the request tx: the buffalo pop transaction middleware rolls back
// the request tx after a redirect response, which would silently discard
// the flag flip. Ensures the delivery/purge worker is running.
func EnableWebhookForwarding(tx *pop.Connection) error {
	persistTx := models.DB
	if persistTx == nil {
		persistTx = tx
	}
	if persistTx == nil {
		return fmt.Errorf("no database connection")
	}
	targets := models.SyncTargets{}
	if err := persistTx.Where("webhook_url <> ''").All(&targets); err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no sync target configured: create a sync target with the console URL before enabling")
	}
	enabled := 0
	for i := range targets {
		if targets[i].Enabled {
			enabled++
			continue
		}
		targets[i].Enabled = true
		verrs, err := persistTx.ValidateAndUpdate(&targets[i])
		if err != nil {
			return err
		}
		if verrs.HasAny() {
			return fmt.Errorf("sync target validation failed: %v", verrs)
		}
		enabled++
	}
	if enabled == 0 {
		return fmt.Errorf("no sync target could be enabled")
	}
	SetSyncTargetsKnown(true)
	EnsureWebhookWorkerRunning()
	return nil
}

// ConfigsResource is the resource for the Config model
type ConfigsResource struct {
	buffalo.Resource
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

// bindSettingsForScope merges submitted form values into the stored settings
// according to the form scope: "sync" updates only the event-stream flag,
// "identity" only center/identity fields, and "" (legacy full form)
// updates both groups. Fields outside the scope keep their stored values.
func bindSettingsForScope(c buffalo.Context, stored models.ConfigSettings, scope string) models.ConfigSettings {
	settings := stored
	if scope == "" || scope == "sync" {
		settings.EnableEventStream = paramIsTrue(c, "Settings.EnableEventStream")
	}
	if scope == "" || scope == "identity" {
		settings.CenterName = c.Param("Settings.CenterName")
		settings.AsblName = c.Param("Settings.AsblName")
		settings.BceNumber = c.Param("Settings.BceNumber")
		settings.Address = c.Param("Settings.Address")
		settings.AccountNumber = c.Param("Settings.AccountNumber")
		settings.Website = c.Param("Settings.Website")
		settings.GuestText1 = c.Param("Settings.GuestText1")
		settings.GuestText2 = c.Param("Settings.GuestText2")
	}
	return settings
}

// bindColumnsForScope merges submitted form values into the config table
// columns according to the form scope: "identity" updates only the
// name/description/active columns, "sync" updates only the instance ID
// (the sync identity moved to the admin sync form), and "" (legacy full
// form) updates both groups. Columns outside the scope keep their stored
// values. Boolean and fields prone to formam parsing issues are bound
// manually.
func bindColumnsForScope(c buffalo.Context, config *models.Config, scope string) {
	if scope != "sync" {
		config.Name = c.Param("Name")
		config.Description = c.Param("Description")
		// Handle Active checkbox manually - it will be "true" if checked, missing if unchecked
		config.Active = paramIsTrue(c, "Active")
	}
	if scope == "" || scope == "sync" {
		config.InstanceID = c.Param("InstanceID")
	}
}

// requireAdmin checks if the current user is an admin
func requireAdmin(c buffalo.Context) (*models.User, error) {
	cu := GetCurrentUser(c)
	if cu == nil || !cu.Admin {
		return nil, c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}
	return cu, nil
}

// requireMaintainer restricts maintainer-only administration surfaces.
func requireMaintainer(c buffalo.Context) (*models.User, error) {
	cu := GetCurrentUser(c)
	if cu == nil || !cu.Admin || !cu.Maintainer {
		return nil, c.Error(http.StatusForbidden, fmt.Errorf("maintainer rights required for this action"))
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
		if CurrentConfigGet() != nil {
			currentConfigID = CurrentConfigGet().ID.String()
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
	settings := models.ConfigSettings{
		EnableEventStream: paramIsTrue(c, "Settings.EnableEventStream"),
		CenterName:        c.Param("Settings.CenterName"),
		AsblName:          c.Param("Settings.AsblName"),
		BceNumber:         c.Param("Settings.BceNumber"),
		Address:           c.Param("Settings.Address"),
		AccountNumber:     c.Param("Settings.AccountNumber"),
		Website:           c.Param("Settings.Website"),
		GuestText1:        c.Param("Settings.GuestText1"),
		GuestText2:        c.Param("Settings.GuestText2"),
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

// SyncEdit renders the synchronization page for the active config: the
// global settings (instance ID, event stream) plus the sync targets list
// with per-target delivery counters and actions (add, edit, delete, retry
// undeliverable). Mapped to GET /sync_configuration (admin Synchronization
// menu) and, for backward compatibility, to GET /config/{config_id}/sync
// (redirects to /sync_configuration when the addressed config is the active
// one).
func (v ConfigsResource) SyncEdit(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	if c.Param("config_id") != "" {
		// Legacy per-config URL: only the active config carries the live
		// synchronization settings, so redirect to the canonical page.
		if CurrentConfigGet() != nil && CurrentConfigGet().ID.String() == c.Param("config_id") {
			return c.Redirect(http.StatusMovedPermanently, "/sync_configuration")
		}
		return c.Error(http.StatusNotFound, fmt.Errorf("synchronization is configured on the active configuration only"))
	}

	if _, err := LoadConfig(tx); err != nil {
		return errors.WithStack(err)
	}
	config := CurrentConfigGet()
	if config == nil {
		return c.Error(http.StatusNotFound, fmt.Errorf("no active configuration"))
	}

	settings, _ := config.GetSettings()
	c.Set("settings", settings)
	c.Set("config", config)

	// Sync targets with their delivery counters (few rows: per-target
	// aggregate queries are fine).
	targets := models.SyncTargets{}
	if err := tx.Order("name asc").All(&targets); err != nil {
		return errors.WithStack(err)
	}
	targetRows := make([]SyncTargetRow, 0, len(targets))
	for i := range targets {
		counts, err := models.DeliveryCountsForTarget(tx, targets[i].ID)
		if err != nil {
			return errors.WithStack(err)
		}
		targetRows = append(targetRows, SyncTargetRow{Target: &targets[i], Counts: counts})
	}
	c.Set("targets", targetRows)

	return c.Render(http.StatusOK, r.HTML("config/sync_edit.plush.html"))
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

	// Scoped update: the identity view (form_scope=identity) edits only the
	// instance identity + center fields; the sync view (form_scope=sync)
	// edits only event-stream/webhook fields. The other field group is
	// preserved from the stored record. No scope = legacy full update.
	scope := c.Param("form_scope")
	stored, storedErr := config.GetSettings()
	if storedErr != nil {
		return errors.WithStack(storedErr)
	}

	// Bind non-boolean fields for the submitted scope; the other field
	// group keeps its stored values (see bindColumnsForScope).
	bindColumnsForScope(c, config, scope)

	settings := bindSettingsForScope(c, stored, scope)
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
	if CurrentConfigGet() != nil && CurrentConfigGet().ID == config.ID {
		CurrentConfigSet(config)
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
	if CurrentConfigGet() != nil && CurrentConfigGet().ID == config.ID {
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
