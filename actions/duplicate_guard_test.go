package actions

import (
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/logger"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// guardLogger returns a quiet logger for direct guard helper calls.
func guardLogger() buffalo.Logger {
	return logger.NewLogger("error")
}

// guardUser creates (or reuses) a user row for veterinaryvisit fixtures.
func guardUser(t *testing.T, tx *pop.Connection) uuid.UUID {
	t.Helper()
	u := &models.User{
		ID:           uuid.Must(uuid.NewV4()),
		Login:        "TS-guard-" + uuid.Must(uuid.NewV4()).String()[:8],
		PasswordHash: "x",
	}
	if err := tx.Create(u); err != nil {
		t.Fatalf("user fixture creation failed: %v", err)
	}
	return u.ID
}

// ageCreatedBackdate moves created_at outside the duplicate window.
func ageCreatedBackdate(t *testing.T, tx *pop.Connection, table, id string) {
	t.Helper()
	if err := tx.RawQuery("UPDATE "+table+" SET created_at = ? WHERE id = ?",
		time.Now().Add(-2*time.Minute-time.Second), id).Exec(); err != nil {
		t.Fatalf("backdating %s.created_at failed: %v", table, err)
	}
}

func TestTreatmentDuplicateGuardFingerprint(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	log := guardLogger()

	tr := &models.Treatment{
		ID:             uuid.Must(uuid.NewV4()),
		Date:           time.Now().Truncate(time.Second),
		AnimalID:       fx.animalC,
		Drug:           "TSDrug-" + fx.marker,
		Dosage:         "1ml",
		Remarks:        nulls.String{},
		Timebitmap:     1,
		Timedonebitmap: 0,
	}
	if err := tx.Create(tr); err != nil {
		t.Fatalf("treatment fixture creation failed: %v", err)
	}

	fp := func(drug, dosage string, remarks nulls.String) []interface{} {
		return []interface{}{tr.AnimalID, tr.Date, drug, dosage, remarks, tr.Timebitmap, tr.Timedonebitmap}
	}

	// identical fingerprint within the window → duplicate detected
	if !recentDuplicateExists(log, tx, &models.Treatment{}, treatmentFingerprintQuery, fp(tr.Drug, tr.Dosage, nulls.String{})...) {
		t.Fatal("expected duplicate detection for identical treatment")
	}

	// different drug → not a duplicate
	if recentDuplicateExists(log, tx, &models.Treatment{}, treatmentFingerprintQuery, fp(tr.Drug+"-x", tr.Dosage, nulls.String{})...) {
		t.Fatal("expected no duplicate detection for different drug")
	}

	// non-NULL remarks vs NULL remarks → not a duplicate (null-safe compare)
	if recentDuplicateExists(log, tx, &models.Treatment{}, treatmentFingerprintQuery, fp(tr.Drug, tr.Dosage, nulls.NewString("note"))...) {
		t.Fatal("expected no duplicate detection for non-NULL remarks vs NULL")
	}

	// created outside the window → not a duplicate
	ageCreatedBackdate(t, tx, "treatments", tr.ID.String())
	if recentDuplicateExists(log, tx, &models.Treatment{}, treatmentFingerprintQuery, fp(tr.Drug, tr.Dosage, nulls.String{})...) {
		t.Fatal("expected no duplicate detection outside the submission window")
	}
}

func TestVeterinaryvisitDuplicateGuardFingerprint(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	log := guardLogger()
	userID := guardUser(t, tx)

	vv := &models.Veterinaryvisit{
		ID:         uuid.Must(uuid.NewV4()),
		Date:       time.Now().Truncate(time.Second),
		UserID:     userID,
		Veterinary: "TSVet-" + fx.marker,
		AnimalID:   fx.animalC,
		Diagnostic: nulls.String{},
	}
	if err := tx.Create(vv); err != nil {
		t.Fatalf("veterinaryvisit fixture creation failed: %v", err)
	}

	fp := func(vet string, diagnostic nulls.String) []interface{} {
		return []interface{}{vv.AnimalID, vv.Date, vet, diagnostic}
	}

	if !recentDuplicateExists(log, tx, &models.Veterinaryvisit{}, veterinaryvisitFingerprintQuery, fp(vv.Veterinary, nulls.String{})...) {
		t.Fatal("expected duplicate detection for identical visit")
	}

	if recentDuplicateExists(log, tx, &models.Veterinaryvisit{}, veterinaryvisitFingerprintQuery, fp(vv.Veterinary+"-x", nulls.String{})...) {
		t.Fatal("expected no duplicate detection for different veterinary")
	}

	if recentDuplicateExists(log, tx, &models.Veterinaryvisit{}, veterinaryvisitFingerprintQuery, fp(vv.Veterinary, nulls.NewString("fracture"))...) {
		t.Fatal("expected no duplicate detection for non-NULL diagnostic vs NULL")
	}

	ageCreatedBackdate(t, tx, "veterinaryvisits", vv.ID.String())
	if recentDuplicateExists(log, tx, &models.Veterinaryvisit{}, veterinaryvisitFingerprintQuery, fp(vv.Veterinary, nulls.String{})...) {
		t.Fatal("expected no duplicate detection outside the submission window")
	}
}

func TestCareDuplicateGuardFingerprint(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	log := guardLogger()

	ct := &models.Caretype{
		ID:   uuid.Must(uuid.NewV4()),
		Name: "TSCaretype-" + fx.marker,
	}
	if err := tx.Create(ct); err != nil {
		t.Fatalf("caretype fixture creation failed: %v", err)
	}

	care := &models.Care{
		ID:        uuid.Must(uuid.NewV4()),
		Date:      time.Now().Truncate(time.Second),
		AnimalID:  fx.animalC,
		TypeID:    ct.ID,
		Weight:    nulls.NewString("150"),
		Note:      nulls.NewString("TS note " + fx.marker),
		Clean:     nulls.Bool{},
		InWarning: nulls.Bool{},
		LinkToID:  nulls.UUID{},
	}
	if err := tx.Create(care); err != nil {
		t.Fatalf("care fixture creation failed: %v", err)
	}

	fp := func(weight, note nulls.String) []interface{} {
		return []interface{}{care.AnimalID, care.Date, care.TypeID, weight, note, care.Clean, care.InWarning, care.LinkToID}
	}

	if !recentDuplicateExists(log, tx, &models.Care{}, careFingerprintQuery, fp(nulls.NewString("150"), nulls.NewString("TS note "+fx.marker))...) {
		t.Fatal("expected duplicate detection for identical care")
	}

	if recentDuplicateExists(log, tx, &models.Care{}, careFingerprintQuery, fp(nulls.NewString("999"), nulls.NewString("TS note "+fx.marker))...) {
		t.Fatal("expected no duplicate detection for different weight")
	}

	// NULL weight vs set weight → not a duplicate (null-safe compare)
	if recentDuplicateExists(log, tx, &models.Care{}, careFingerprintQuery, fp(nulls.String{}, nulls.NewString("TS note "+fx.marker))...) {
		t.Fatal("expected no duplicate detection for NULL weight vs set weight")
	}

	ageCreatedBackdate(t, tx, "cares", care.ID.String())
	if recentDuplicateExists(log, tx, &models.Care{}, careFingerprintQuery, fp(nulls.NewString("150"), nulls.NewString("TS note "+fx.marker))...) {
		t.Fatal("expected no duplicate detection outside the submission window")
	}
}
