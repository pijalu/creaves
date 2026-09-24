package actions

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// loadOuttakeCageAnimals returns the animals of a cage that still need an
// outtake (no outtake linked yet), eager-loading the intake graph so stay
// durations can be computed per animal (issue #170).
func loadOuttakeCageAnimals(tx *pop.Connection, cage string) (*models.Animals, error) {
	animals := &models.Animals{}
	if err := tx.Eager().Where("outtake_id is null and Cage = ?", cage).All(animals); err != nil {
		return nil, err
	}
	return animals, nil
}

// OuttakeCageNew renders the batch outtake flow for a whole cage (issue
// #170). Without a ?cage= param it renders the cage picker; with a cage it
// renders the shared outtake form. The first animal of the cage is the
// reference: its intake date feeds the stay-duration helpers and its species
// native status filters the selectable outtake types — the chosen rules then
// apply to the whole cage.
func OuttakeCageNew(c buffalo.Context) error {
	c.Set("intakeDate", "")
	cage := c.Param("cage")
	if len(cage) == 0 {
		outtake := &models.Outtake{
			Date: time.Now(),
		}
		c.Set("outtake", outtake)
		return c.Render(http.StatusOK, r.HTML("/outtakes/cage_new.plush.html"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals, err := loadOuttakeCageAnimals(tx, cage)
	if err != nil {
		return err
	}
	if len(*animals) == 0 {
		c.Flash().Add("warning", T.Translate(c, "outtake.cage.empty"))
		return c.Redirect(http.StatusSeeOther, "/outtakes/cage")
	}

	first := (*animals)[0]
	outtake := &models.Outtake{
		Date: time.Now(),
	}
	c.Set("outtake", outtake)
	c.Set("cageAnimals", animals)
	c.Set("cageAnimalCount", len(*animals))
	c.Set("intakeDate", first.Intake.Date.Format(models.DateTimeFormat))

	ot, err := outtakeTypes(c)
	if err != nil {
		return err
	}
	// Type choice follows the first animal's rules (native status).
	if err := setFilteredOuttakeFormData(c, tx, ot, &first); err != nil {
		return err
	}

	return c.Render(http.StatusOK, r.HTML("/outtakes/cage_new.plush.html"))
}

// cageOuttakeAccepted applies the first-animal rules to the shared batch
// fields: no future date, existing type, type allowed for the reference
// species, location matching the type mode. Returns false (after flashing
// the reason) when the batch must be rejected.
func cageOuttakeAccepted(c buffalo.Context, tx *pop.Connection, first *models.Animal, outtake *models.Outtake) bool {
	if outtake.Date.After(models.FormWallClockNow().Add(time.Minute)) {
		c.Flash().Add("danger", T.Translate(c, "outtake.date.future"))
		return false
	}
	outtakeType := &models.Outtaketype{}
	if err := tx.Find(outtakeType, outtake.TypeID); err != nil {
		c.Flash().Add("danger", T.Translate(c, "outtake.type.invalid"))
		return false
	}
	if outtakeType.ExcludesNativeStatus(speciesNativeStatus(tx, first.Species)) {
		c.Flash().Add("danger", T.Translate(c, "outtake.type.forbidden_by_native_status"))
		return false
	}
	outtake.Type = *outtakeType
	if err := enforceOuttakeLocationRule(tx, outtake, outtakeType); err != nil {
		c.Flash().Add("danger", T.Translate(c, "outtake.location.invalid"))
		return false
	}
	return true
}

// createOuttakeForAnimal persists one outtake for an animal of the cage with
// its own stay duration, then applies the same side effects as the
// single-outtake create: future-treatment cleanup (audited), animal link,
// audit trail and lifecycle events.
func createOuttakeForAnimal(c buffalo.Context, tx *pop.Connection, a *models.Animal, shared *models.Outtake, outtakeType *models.Outtaketype) error {
	o := *shared // shared fields: date, type, location, note
	o.ID = uuid.Nil
	o.CreatedAt = time.Time{}
	o.UpdatedAt = time.Time{}
	o.StayDuration = nulls.Int{}
	o.ComputeStayDuration(a.Intake.Date)

	verrs, err := tx.ValidateAndCreate(&o)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		msgs := make([]string, 0, len(verrs.Errors))
		for _, m := range verrs.Errors {
			msgs = append(msgs, strings.Join(m, ", "))
		}
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}

	oldAnimal := *a
	deletedTreatments := models.Treatments{}
	if err := tx.Where("animal_id = ? and date >= ?", a.ID, o.Date).All(&deletedTreatments); err != nil {
		return err
	}
	a.OuttakeID = nulls.NewUUID(o.ID)
	a.Outtake = &o
	if err := tx.Update(a); err != nil {
		return err
	}
	if err := tx.RawQuery("DELETE FROM treatments WHERE animal_id = ? and date >= ?",
		a.ID, o.Date).Exec(); err != nil {
		return err
	}

	auditAnimalChange(c, tx, a.ID, models.AuditEntityOuttake, auditEntityID(o.ID), models.AuditActionCreate, nil, auditOuttakeProjection(o))
	auditAnimalChange(c, tx, a.ID, models.AuditEntityAnimal, auditEntityID(a.ID), models.AuditActionUpdate, auditAnimalProjection(oldAnimal), auditAnimalProjection(*a))
	for j := range deletedTreatments {
		auditAnimalChange(c, tx, a.ID, models.AuditEntityTreatment, auditEntityID(deletedTreatments[j].ID), models.AuditActionDelete, auditTreatmentProjection(deletedTreatments[j]), nil)
	}

	if outtakeType.Error {
		if err := PublishAnimalDiedEvent(tx, a, GetCurrentUser(c)); err != nil {
			c.Logger().Warnf("Failed to publish animal_died event: %v", err)
		}
	} else {
		if err := PublishAnimalReleasedEvent(tx, a, GetCurrentUser(c)); err != nil {
			c.Logger().Warnf("Failed to publish animal_released event: %v", err)
		}
	}
	publishAnimalStateEventWarn(c, tx, a.ID)
	return nil
}

// OuttakeCageCreate saves one outtake for every animal of the cage that
// still needs one (issue #170). The outtake type and rules are chosen from
// the first animal and applied to the whole cage.
func OuttakeCageCreate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	cage := c.Param("cage")
	if len(cage) == 0 {
		return c.Redirect(http.StatusSeeOther, "/outtakes/cage")
	}

	animals, err := loadOuttakeCageAnimals(tx, cage)
	if err != nil {
		return err
	}
	if len(*animals) == 0 {
		c.Flash().Add("warning", T.Translate(c, "outtake.cage.empty"))
		return c.Redirect(http.StatusSeeOther, "/outtakes/cage")
	}
	first := (*animals)[0]

	// Shared outtake fields bound from the form.
	outtake := &models.Outtake{}
	if err := c.Bind(outtake); err != nil {
		return err
	}

	// Reject the whole batch up front if the shared fields violate the
	// first animal's rules.
	if !cageOuttakeAccepted(c, tx, &first, outtake) {
		return cageRejectRedirect(c, cage)
	}

	saved := 0
	for i := range *animals {
		a := &(*animals)[i]
		if err := createOuttakeForAnimal(c, tx, a, outtake, &outtake.Type); err != nil {
			// Validations are identical for every animal (same shared
			// fields); realistically this fires before anything was saved.
			c.Flash().Add("danger", T.Translate(c, "outtake.cage.failed", map[string]interface{}{
				"error": err.Error(),
			}))
			return cageRejectRedirect(c, cage)
		}
		saved++
	}

	InvalidateAnnualStatsCache()
	c.Flash().Add("success", T.Translate(c, "outtake.cage.success", map[string]interface{}{
		"count": saved,
		"cage":  cage,
	}))
	return c.Redirect(http.StatusSeeOther, "/outtakes/")
}

// cageRejectRedirect sends the user back to the cage form keeping the
// selected cage.
func cageRejectRedirect(c buffalo.Context, cage string) error {
	return c.Redirect(http.StatusSeeOther, "/outtakes/cage?cage="+url.QueryEscape(cage))
}
