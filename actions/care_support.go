package actions

import (
	"creaves/models"
	"strings"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// previousCareWeight returns the most recent weighted care for the animal
// strictly before `before` (used by the ±10% weight warning in the care form,
// issue #158). excludeID skips the care being edited. Returns an invalid
// nulls.String when no previous weight exists.
//
// The DSN has no loc parameter, so the driver formats time.Time args in UTC,
// while care dates coming from forms are stored as local wall-clock values.
// Normalize `before` to those wall-clock components (labelled UTC) so the
// comparison is done in stored-value terms and not shifted by the TZ offset.
func previousCareWeight(tx *pop.Connection, animalID int, before time.Time, excludeID nulls.UUID) nulls.String {
	before = time.Date(before.Year(), before.Month(), before.Day(), before.Hour(), before.Minute(), before.Second(), 0, time.UTC)
	q := tx.Where("animal_id = ? AND date < ? AND weight IS NOT NULL AND weight <> ''", animalID, before)
	if excludeID.Valid {
		q = q.Where("id <> ?", excludeID)
	}
	c := &models.Care{}
	if err := q.Order("date desc").First(c); err != nil {
		return nulls.String{}
	}
	return c.Weight
}

// soinCareTypeID finds the "Soin" care type (case-insensitive canonical name).
func soinCareTypeID(tx *pop.Connection) (uuid.UUID, bool) {
	ct := &models.Caretype{}
	if err := tx.Where("LOWER(name) = ?", "soin").First(ct); err != nil {
		return uuid.Nil, false
	}
	return ct.ID, true
}

// suiviCareTypeID finds the "Suivi" care type (case-insensitive canonical name).
func suiviCareTypeID(tx *pop.Connection) (uuid.UUID, bool) {
	ct := &models.Caretype{}
	if err := tx.Where("LOWER(name) = ?", "suivi").First(ct); err != nil {
		return uuid.Nil, false
	}
	return ct.ID, true
}

// supportCareNote builds the note of the automatic "Soin" care recording
// heat source / oxygen support (issue #158). Canonical French wording.
func supportCareNote(heat nulls.String, oxygen bool) string {
	parts := []string{}
	if heat.Valid && strings.TrimSpace(heat.String) != "" {
		parts = append(parts, "Source de chaleur : "+strings.TrimSpace(heat.String))
	}
	if oxygen {
		parts = append(parts, "O2")
	}
	return strings.Join(parts, " / ")
}

// careNeedsSupportCare reports whether the heat source / oxygen fields were
// used on the care (issue #158: these edits add a "Soin" care automatically).
func careNeedsSupportCare(care *models.Care) bool {
	return (care.HeatSource.Valid && strings.TrimSpace(care.HeatSource.String) != "") || care.Oxygen
}

// createAutoSupportCare adds a "Soin" care mirroring the heat source / oxygen
// fields of `care` (issue #158). Best effort: missing "Soin" care type or a
// validation error is logged, never fails the user's own care save. The
// duplicate submission guard also protects this auxiliary record.
func createAutoSupportCare(c buffalo.Context, tx *pop.Connection, care *models.Care) {
	if !careNeedsSupportCare(care) {
		return
	}
	typeID, ok := soinCareTypeID(tx)
	if !ok {
		c.Logger().Warn("createAutoSupportCare: no 'Soin' care type found; skipping automatic support care")
		return
	}

	sc := &models.Care{
		Date:       care.Date,
		AnimalID:   care.AnimalID,
		TypeID:     typeID,
		HeatSource: care.HeatSource,
		Oxygen:     care.Oxygen,
		Note:       nulls.NewString(supportCareNote(care.HeatSource, care.Oxygen)),
	}
	if recentDuplicateExists(c.Logger(), tx, &models.Care{}, careFingerprintQuery,
		sc.AnimalID, sc.Date, sc.TypeID, sc.Weight, sc.Note, sc.Clean, sc.InWarning, sc.LinkToID, sc.HeatSource, sc.Oxygen) {
		return
	}
	if verrs, err := tx.ValidateAndCreate(sc); err != nil || verrs.HasAny() {
		c.Logger().Warnf("createAutoSupportCare: %v %v", verrs, err)
		return
	}
	auditAnimalChange(c, tx, care.AnimalID, models.AuditEntityCare, auditEntityID(sc.ID), models.AuditActionCreate, nil, auditCareProjection(*sc))
}

// SuggestionsHeatSource autocompletes the heat source field from values
// already recorded on cares (issue #158).
func SuggestionsHeatSource(c buffalo.Context) error {
	return suggest(c, "cares", "heat_source")
}
