package actions

import (
	"fmt"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

// auditAnimalChange records an audit entry for a change concerning an animal.
// Audit logging is best-effort: failures are logged but never abort the
// business operation in progress.
//
//	oldRec / newRec are model snapshots (either may be nil for create/delete);
//	they are diffed to build the human readable change summary.
func auditAnimalChange(c buffalo.Context, tx *pop.Connection, animalID int, entity, entityID, action string, oldRec, newRec interface{}) {
	if animalID == 0 {
		c.Logger().Errorf("audit: skip %s %s change with zero animal id", entity, action)
		return
	}

	changes := models.ComputeChanges(oldRec, newRec)
	if auditSkipEmptyUpdate(action, changes) {
		return
	}
	entry := &models.AnimalAudit{
		AnimalID: animalID,
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		Changes:  changes,
	}
	if user := GetCurrentUser(c); user != nil {
		entry.UserID = nulls.NewUUID(user.ID)
		entry.UserName = user.Login
	} else {
		entry.UserName = "system"
	}

	if err := tx.Create(entry); err != nil {
		c.Logger().Errorf("audit: failed to record %s %s change for animal %d: %v", entity, action, animalID, err)
	}
}

// auditEntityID formats an entity primary key for the audit log.
func auditEntityID(id interface{}) string {
	return fmt.Sprintf("%v", id)
}

// auditAnimalChange returns true when the entry would be an update without
// any detectable change; such entries are skipped to keep the log clean.
func auditSkipEmptyUpdate(action, changes string) bool {
	return action == models.AuditActionUpdate && changes == ""
}

// auditAnimalProjection returns a copy of the animal restricted to the
// animal-table columns: nested associations are zeroed (they are audited as
// their own entities) while the foreign key columns are kept.
func auditAnimalProjection(a models.Animal) models.Animal {
	a.Animalage = models.Animalage{}
	a.Animaltype = models.Animaltype{}
	a.Discovery = models.Discovery{}
	a.Intake = models.Intake{}
	a.Outtake = nil
	a.Cares = nil
	a.Treatments = nil
	a.VetVisits = nil
	return a
}

// auditDiscoveryProjection zeroes nested association structs, keeping the
// foreign key columns.
func auditDiscoveryProjection(d models.Discovery) models.Discovery {
	d.EntryCause = models.EntryCause{}
	d.Discoverer = models.Discoverer{}
	return d
}

// auditOuttakeProjection zeroes the nested outtake type struct.
func auditOuttakeProjection(o models.Outtake) models.Outtake {
	o.Type = models.Outtaketype{}
	return o
}

// auditCareProjection zeroes nested association structs.
func auditCareProjection(ca models.Care) models.Care {
	ca.Animal = models.Animal{}
	ca.Type = models.Caretype{}
	ca.LinkTo = nil
	return ca
}

// auditTreatmentProjection zeroes nested association structs.
func auditTreatmentProjection(t models.Treatment) models.Treatment {
	t.Animal = nil
	return t
}

// auditVeterinaryvisitProjection zeroes nested association structs.
func auditVeterinaryvisitProjection(v models.Veterinaryvisit) models.Veterinaryvisit {
	v.User = models.User{}
	v.Animal = models.Animal{}
	return v
}

// auditTravelProjection zeroes nested association structs.
func auditTravelProjection(t models.Travel) models.Travel {
	t.Animal = nil
	t.User = nil
	t.Traveltype = nil
	return t
}

// auditAnimalIDByIntake resolves the animal linked to an intake via the
// animals.intake_id column. Returns 0 when no animal is linked.
func auditAnimalIDByIntake(tx *pop.Connection, intakeID interface{}) int {
	a := &models.Animal{}
	if err := tx.Where("intake_id = ?", intakeID).First(a); err == nil {
		return a.ID
	}
	return 0
}

// auditAnimalIDByDiscovery resolves the animal linked to a discovery via the
// animals.discovery_id column. Returns 0 when no animal is linked.
func auditAnimalIDByDiscovery(tx *pop.Connection, discoveryID interface{}) int {
	a := &models.Animal{}
	if err := tx.Where("discovery_id = ?", discoveryID).First(a); err == nil {
		return a.ID
	}
	return 0
}
