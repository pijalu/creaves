package actions

import (
	"fmt"
	"net/http"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

// Animal plan HTTP layer (§4.7, §7.2): caretaker-level CRUD nested under
// the animal. No admin gate — any approved account may plan a single
// animal; RoleGuard already strips write access from restricted roles.

// CareAnimalPlanList handles GET /animals/{animal_id}/care_animal_plans:
// the animal's own plans (Plan tab, §7.2).
func CareAnimalPlanList(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	plans := &models.CareAnimalPlans{}
	if err := tx.Where("animal_id = ?", animal.ID).All(plans); err != nil {
		return err
	}
	return c.Render(http.StatusOK, renderJSON(plans))
}

// CareAnimalPlanCreate handles POST /animals/{animal_id}/care_animal_plans
// (§7.2): same action/schedule payloads as rules (§4.7 parity).
func CareAnimalPlanCreate(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	plan := &models.CareAnimalPlan{}
	if err := c.Bind(plan); err != nil {
		return err
	}
	// Repeat is the source of truth (BUG-36 decision 3): times re-derived
	// server-side before validation/save.
	sched, nerr := normalizeSubmittedSchedule(plan.Schedule)
	if nerr != nil {
		return scheduleFieldError(c, nerr.Error())
	}
	plan.Schedule = sched
	u := GetCurrentUser(c)
	if u != nil {
		plan.CreatedBy = nulls.NewUUID(u.ID)
	}
	plan.AnimalID = animal.ID
	plan.Active = true // §4.7: caretaker-created plans are active by default
	verrs, err := tx.ValidateAndCreate(plan)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return respondPlanVerrs(c, verrs)
	}
	return c.Render(http.StatusCreated, renderJSON(plan))
}

// CareAnimalPlanUpdate handles PUT
// /animals/{animal_id}/care_animal_plans/{care_animal_plan_id}.
func CareAnimalPlanUpdate(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	plan := &models.CareAnimalPlan{}
	if err := tx.Find(plan, c.Param("care_animal_plan_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if plan.AnimalID != animal.ID {
		return planError(c, http.StatusNotFound, fmt.Errorf("plan does not belong to animal"))
	}
	if err := c.Bind(plan); err != nil {
		return err
	}
	// Repeat is the source of truth (BUG-36 decision 3): times re-derived
	// server-side before validation/save.
	sched, nerr := normalizeSubmittedSchedule(plan.Schedule)
	if nerr != nil {
		return scheduleFieldError(c, nerr.Error())
	}
	plan.Schedule = sched
	// Confirm-on-save: a human editing the plan IS the review the
	// converter asked for — drop the " (à vérifier)" fallback flag so the
	// animal page NeedsReview badge clears (§8.1). " (conversion)" stays:
	// it pairs with created_by IS NULL for the §8.2 rollback targeting.
	plan.Name = strings.TrimSpace(strings.ReplaceAll(plan.Name, " (à vérifier)", ""))
	// path is authoritative
	plan.AnimalID = animal.ID
	if id, err := uuid.FromString(c.Param("care_animal_plan_id")); err == nil {
		plan.ID = id
	}
	verrs, err := tx.ValidateAndUpdate(plan)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return respondPlanVerrs(c, verrs)
	}
	return c.Render(http.StatusOK, renderJSON(plan))
}

// CareAnimalPlanDestroy handles DELETE
// /animals/{animal_id}/care_animal_plans/{care_animal_plan_id}.
func CareAnimalPlanDestroy(c buffalo.Context) error {
	tx := planTx(c)
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	plan := &models.CareAnimalPlan{}
	if err := tx.Find(plan, c.Param("care_animal_plan_id")); err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	if plan.AnimalID != animal.ID {
		return planError(c, http.StatusNotFound, fmt.Errorf("plan does not belong to animal"))
	}
	if err := tx.Destroy(plan); err != nil {
		return err
	}
	return c.Render(http.StatusOK, renderJSON(map[string]string{"status": "deleted"}))
}
