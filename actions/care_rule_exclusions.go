package actions

import (
	"fmt"
	"net/http"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// R4-7.13 — per-animal protocol exception (§4.1), the write side.
//
// care_rule_exclusions existed in the model and was honoured by the day-plan
// engine (care_plan_dayplan.go) but nothing could ever CREATE one, and the
// protocol trace listed only sources that produced occurrences — so an
// exclusion was invisible in both directions: impossible to set, and once set
// impossible to find or undo.
//
// Permission: no admin gate, on purpose. AnimalsResource.Update has none
// either — this app lets any approved account edit an animal, so an animal's
// exception follows the animal. RoleGuard still blocks restricted accounts,
// as it does for the animal-plan routes.

type careRuleExclusionInput struct {
	RuleID string `json:"rule_id"`
	Reason string `json:"reason"`
}

// CareRuleExclusionCreate handles POST
// /animals/{animal_id}/care_rule_exclusions: opt THIS animal out of a global
// care rule, optionally recording why.
//
// 409 when the exclusion already exists — the trace renders one row per rule,
// so a duplicate would give the caregiver two "restore" buttons for one rule.
func CareRuleExclusionCreate(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	in := &careRuleExclusionInput{}
	if err := c.Bind(in); err != nil {
		return err
	}
	ruleID, err := uuid.FromString(in.RuleID)
	if err != nil {
		return planError(c, http.StatusUnprocessableEntity, err)
	}
	rule := &models.CareRule{}
	if err := tx.Find(rule, ruleID); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if countRuleExclusions(tx, ruleID, animal.ID) > 0 {
		return planError(c, http.StatusConflict, errAlreadyExcluded)
	}
	exclusion := &models.CareRuleExclusion{
		RuleID:   ruleID,
		AnimalID: animal.ID,
		Reason:   nulls.NewString(in.Reason),
	}
	if u := GetCurrentUser(c); u != nil {
		exclusion.CreatedBy = uuid.NullUUID{UUID: u.ID, Valid: true}
	}
	verrs, err := tx.ValidateAndCreate(exclusion)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return c.Render(http.StatusUnprocessableEntity, renderJSON(verrs))
	}
	return c.Render(http.StatusCreated, renderJSON(exclusion))
}

// CareRuleExclusionDestroy handles DELETE
// /animals/{animal_id}/care_rule_exclusions/{care_rule_exclusion_id}: restore
// the rule for this animal. Scoped to the animal — an exclusion belonging to
// another animal is a 404, never a cross-animal delete.
func CareRuleExclusionDestroy(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	exclusion := &models.CareRuleExclusion{}
	if err := tx.Find(exclusion, c.Param("care_rule_exclusion_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if exclusion.AnimalID != animal.ID {
		return planError(c, http.StatusNotFound, errExclusionOtherAnimal)
	}
	if err := tx.Destroy(exclusion); err != nil {
		return err
	}
	return c.Render(http.StatusOK, renderJSON(map[string]string{"status": "deleted"}))
}

var (
	errAlreadyExcluded      = fmt.Errorf("animal is already excluded from this care rule")
	errExclusionOtherAnimal = fmt.Errorf("exclusion does not belong to animal")
)

// countRuleExclusions is the duplicate guard; a COUNT never needs an alias.
func countRuleExclusions(tx *pop.Connection, ruleID uuid.UUID, animalID int) int {
	n, err := tx.Where("rule_id = ? AND animal_id = ?", ruleID, animalID).Count(&models.CareRuleExclusions{})
	if err != nil {
		return 1 // fail closed: never create a second row we cannot see
	}
	return n
}
