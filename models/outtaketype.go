package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Outtake location modes: how the outtake Location field is handled on the
// new-outtake form. "none" hides and clears the field, "free" keeps the
// free-text input with suggestions, "list" restricts the value to the
// outtake_location_options reference list.
const (
	OuttakeLocationModeNone = "none"
	OuttakeLocationModeFree = "free"
	OuttakeLocationModeList = "list"
)

// Outtaketype is used by pop to map your outtaketypes database table to your go code.
type Outtaketype struct {
	ID                     uuid.UUID    `json:"id" db:"id"`
	Name                   string       `json:"name" db:"name"`
	Code                   nulls.String `json:"code" db:"code"`
	Default                bool         `json:"default" db:"def"`
	Dead                   bool         `json:"dead" db:"dead"`
	Error                  bool         `json:"error" db:"error"`
	Rating                 int          `json:"rating" db:"rating"`
	Description            nulls.String `json:"description" db:"description"`
	DiscovererNews         nulls.String `json:"discoverer_news" db:"discoverer_news"`
	ExcludedNativeStatuses nulls.String `json:"excluded_native_statuses" db:"excluded_native_statuses"`
	LocationMode           string       `json:"location_mode" db:"location_mode"`
	CreatedAt              time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (o Outtaketype) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// Outtaketypes is not required by pop and may be deleted
type Outtaketypes []Outtaketype

// String is not required by pop and may be deleted
func (o Outtaketypes) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (o *Outtaketype) Validate(tx *pop.Connection) (*validate.Errors, error) {
	o.ExcludedNativeStatuses = normalizeNativeStatusCSV(o.ExcludedNativeStatuses)
	if o.LocationMode == "" {
		o.LocationMode = OuttakeLocationModeNone
	}
	return validate.Validate(
		&validators.StringIsPresent{Field: o.Name, Name: "Name"},
		&validators.StringInclusion{Field: o.LocationMode, Name: "LocationMode", List: []string{OuttakeLocationModeNone, OuttakeLocationModeFree, OuttakeLocationModeList}},
	), nil
}

// ExcludedNativeStatusList returns the parsed list of excluded native
// status IDs from the stored CSV value.
func (o Outtaketype) ExcludedNativeStatusList() []string {
	if !o.ExcludedNativeStatuses.Valid {
		return nil
	}
	res := []string{}
	for _, id := range strings.Split(o.ExcludedNativeStatuses.String, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			res = append(res, id)
		}
	}
	return res
}

// ExcludesNativeStatus reports whether this outtake type is forbidden for a
// species with the given native status ID.
func (o Outtaketype) ExcludesNativeStatus(nativeStatus string) bool {
	nativeStatus = strings.TrimSpace(nativeStatus)
	if nativeStatus == "" {
		return false
	}
	for _, id := range o.ExcludedNativeStatusList() {
		if id == nativeStatus {
			return true
		}
	}
	return false
}

// normalizeNativeStatusCSV canonicalizes a CSV of native status IDs: trims
// spaces, drops empty items, returns NULL for an effectively empty list.
func normalizeNativeStatusCSV(csv nulls.String) nulls.String {
	if !csv.Valid {
		return csv
	}
	ids := []string{}
	for _, id := range strings.Split(csv.String, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nulls.String{}
	}
	return nulls.NewString(strings.Join(ids, ","))
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (o *Outtaketype) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (o *Outtaketype) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
