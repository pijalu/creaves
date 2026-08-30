package models

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// Audit entity names used in the animal_audits table.
const (
	AuditEntityAnimal          = "animal"
	AuditEntityCare            = "care"
	AuditEntityTreatment       = "treatment"
	AuditEntityVeterinaryVisit = "veterinaryvisit"
	AuditEntityTravel          = "travel"
	AuditEntityIntake          = "intake"
	AuditEntityOuttake         = "outtake"
	AuditEntityDiscovery       = "discovery"
	AuditEntityDiscoverer      = "discoverer"
)

// Audit action names.
const (
	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
)

// AnimalAudit records a single auditable change concerning one animal.
// Every mutation of an animal or one of its sub-entities (cares, treatments,
// veterinary visits, travels, intake, outtake, discovery) produces one row.
type AnimalAudit struct {
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Animal concerned by the change.
	AnimalID int `json:"animal_id" db:"animal_id"`

	// User who made the change. UserID may be null for system-initiated
	// changes; UserName is denormalized so the log stays readable even if
	// the user account is later deleted.
	UserID   nulls.UUID `json:"user_id" db:"user_id"`
	UserName string     `json:"user_name" db:"user_name"`

	// What was changed: one of the AuditEntity* constants.
	Entity string `json:"entity" db:"entity"`
	// Primary key of the changed record (as string to support both int and
	// uuid sub-entity keys).
	EntityID string `json:"entity_id" db:"entity_id"`

	// create / update / delete.
	Action string `json:"action" db:"action"`

	// Human readable summary of the change ("field: old -> new; ...").
	Changes string `json:"changes" db:"changes"`
}

// String is not required by pop and may be deleted
func (a AnimalAudit) String() string {
	jl, _ := json.Marshal(a)
	return string(jl)
}

// AnimalAudits is not required by pop and may be deleted
type AnimalAudits []AnimalAudit

// String is not required by pop and may be deleted
func (a AnimalAudits) String() string {
	jl, _ := json.Marshal(a)
	return string(jl)
}

// CreatedAtFormated returns a formated date
func (a AnimalAudit) CreatedAtFormated() string {
	return a.CreatedAt.Format(DateTimeFormat)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
func (a *AnimalAudit) Validate(tx *pop.Connection) (*validate.Errors, error) {
	var errs *validate.Errors = validate.NewErrors()
	if a.AnimalID == 0 {
		errs.Add("animal_id", "AnimalID must not be zero")
	}
	if strings.TrimSpace(a.UserName) == "" {
		errs.Add("user_name", "UserName must not be blank")
	}
	if strings.TrimSpace(a.Entity) == "" {
		errs.Add("entity", "Entity must not be blank")
	}
	if strings.TrimSpace(a.Action) == "" {
		errs.Add("action", "Action must not be blank")
	}
	return errs, nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
func (a *AnimalAudit) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
func (a *AnimalAudit) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// fields excluded from the change diff: bookkeeping columns that always
// change together with any real modification.
var auditIgnoredFields = map[string]bool{
	"created_at": true,
	"updated_at": true,
}

// ComputeChanges builds a human readable summary of the differences between
// two model snapshots. Either side may be nil (record creation / deletion).
// The result is a semicolon separated list of "field: old -> new" items,
// or an empty string when both snapshots are equal.
func ComputeChanges(oldRec, newRec interface{}) string {
	var oldMap, newMap map[string]interface{}

	if oldRec != nil {
		b, err := json.Marshal(oldRec)
		if err == nil {
			_ = json.Unmarshal(b, &oldMap)
		}
	}
	if newRec != nil {
		b, err := json.Marshal(newRec)
		if err == nil {
			_ = json.Unmarshal(b, &newMap)
		}
	}

	if oldMap == nil && newMap == nil {
		return ""
	}

	keys := make(map[string]bool)
	for k := range oldMap {
		keys[k] = true
	}
	for k := range newMap {
		keys[k] = true
	}

	sorted := make([]string, 0, len(keys))
	for k := range keys {
		if auditIgnoredFields[k] {
			continue
		}
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var parts []string
	for _, k := range sorted {
		oldVal, hasOld := oldMap[k]
		newVal, hasNew := newMap[k]

		if hasOld != hasNew {
			if hasOld {
				parts = append(parts, fmt.Sprintf("%s: %s -> <deleted>", k, auditFmt(oldVal)))
			} else {
				parts = append(parts, fmt.Sprintf("%s: <none> -> %s", k, auditFmt(newVal)))
			}
			continue
		}

		if !auditValuesEqual(oldVal, newVal) {
			parts = append(parts, fmt.Sprintf("%s: %s -> %s", k, auditFmt(oldVal), auditFmt(newVal)))
		}
	}

	return strings.Join(parts, "; ")
}

// auditValuesEqual compares two decoded JSON values for equality.
func auditValuesEqual(a, b interface{}) bool {
	// JSON numbers decode as float64; compare via canonical JSON encoding to
	// avoid float formatting mismatches.
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

// auditFmt renders a decoded JSON value compactly for the change summary.
func auditFmt(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "<null>"
	case string:
		if t == "" {
			return "<empty>"
		}
		return t
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// LogAnimalAudit records an audit entry for a change on the given animal.
// It is best-effort: a logging failure is returned but must never abort the
// business operation of the caller — handlers decide how to surface it.
// user may be nil for system-initiated changes.
func LogAnimalAudit(tx *pop.Connection, user *User, animalID int, entity, entityID, action, changes string) error {
	if tx == nil {
		return fmt.Errorf("LogAnimalAudit: nil connection")
	}
	if animalID == 0 {
		return fmt.Errorf("LogAnimalAudit: animalID must not be zero")
	}

	entry := &AnimalAudit{
		AnimalID: animalID,
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		Changes:  changes,
	}
	if user != nil {
		entry.UserID = nulls.NewUUID(user.ID)
		entry.UserName = user.Login
	} else {
		entry.UserName = "system"
	}

	return tx.Create(entry)
}
