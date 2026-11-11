package models

import (
	"creaves/utils"
	"encoding/json"
	"sort"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Treatment is used by pop to map your treatments database table to your go code.
type Treatment struct {
	ID             uuid.UUID    `json:"id" db:"id"`
	Date           time.Time    `json:"date" db:"date"`
	AnimalID       int          `json:"animal_id" db:"animal_id"`
	Animal         *Animal      `json:"-" belongs_to:"animal"`
	Drug           string       `json:"drug" db:"drug"`
	Dosage         string       `json:"dosage" db:"dosage"`
	Remarks        nulls.String `json:"remarks" db:"remarks"`
	Timebitmap     int          `json:"timebitmap" db:"timebitmap"`
	Timedonebitmap int          `json:"timedonebitmap" db:"timedonebitmap"`
	// Entries are the per-time expected administrations (bugs.md R5-3a,
	// U25/D-e) — the source of truth for done state since R5-3. The legacy
	// Timedonebitmap above is dormant (no reads/writes in live paths).
	Entries   TreatmentTimeEntries `json:"-" has_many:"treatment_time_entries" order_by:"due_at asc"`
	CreatedAt time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt time.Time            `json:"updated_at" db:"updated_at"`
}

// Helper structure for presentation
type TreatmentKey struct {
	Date    time.Time
	DateFmt string
	Past    bool
	Current bool
	Future  bool
}

// TreatmentStatusStatistics return statistic on the treatments
type TreatmentStatusStatistics struct {
	//Morning null if treatment not required, true or false if done
	Morning nulls.Bool
	//Noon null if treatment not required, true or false if done
	Noon nulls.Bool
	//Evening null if treatment not required, true or false if done
	Evening nulls.Bool
}

// TreatmentTemplate is the object used to create a serie of treaments
type TreatmentTemplate struct {
	Dates string `json:"dates"`

	AnimalID int          `json:"animal_id"`
	Animal   *Animal      `json:"-" belongs_to:"animal"`
	Drug     string       `json:"drug"`
	Dosage   string       `json:"dosage"`
	Remarks  nulls.String `json:"remarks"`

	// NoDrug marks a wound-care treatment (issue #96): no medication and no
	// posology, only remarks. The stored Drug is WoundCareDrugName.
	NoDrug bool `json:"no_drug"`

	Morning bool `json:"morning"`
	Noon    bool `json:"noon"`
	Evening bool `json:"evening"`
}

// WoundCareDrugName is the canonical Drug value stored for wound-care
// treatments that carry no medication and no posology (issue #96). Such
// treatments consist of remarks only; the Dosage requirement is waived in
// validation when Drug has this value.
const WoundCareDrugName = "Soin de plaie"

// Time bitmaps
const (
	Treatement_MORNING = 1
	Treatement_NOON    = 2
	Treatement_EVENING = 4
)

func TreatmentBoolToBitmap(morning bool, noon bool, evening bool) int {
	bitmap := 0
	if morning {
		bitmap += Treatement_MORNING
	}
	if noon {
		bitmap += Treatement_NOON
	}
	if evening {
		bitmap += Treatement_EVENING
	}
	return bitmap
}

// String is not required by pop and may be deleted
func (t *TreatmentTemplate) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// String is not required by pop and may be deleted
func (t Treatment) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Treatments is not required by pop and may be deleted
type Treatments []Treatment

// String is not required by pop and may be deleted
func (t Treatments) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// TreatmentsMap is an organized treaments list
type TreatmentsMap map[TreatmentKey]Treatments

// Return orderedkeys from map
func (t TreatmentsMap) OrderedKeys() []TreatmentKey {
	var keys []TreatmentKey
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Date.After(keys[j].Date)
	})
	return keys
}

func and(b *nulls.Bool, ba nulls.Bool) {
	if b.Valid {
		if ba.Valid {
			*b = nulls.NewBool(b.Bool && ba.Bool)
		}
	} else {
		*b = ba
	}
}

func (ts Treatments) TodayStatitics() TreatmentStatusStatistics {
	stat := TreatmentStatusStatistics{}

	for _, t := range ts {
		and(&stat.Morning, t.ScheduleStatusMorning())
		and(&stat.Noon, t.ScheduleStatusNoon())
		and(&stat.Evening, t.ScheduleStatusEvening())
	}

	return stat
}

// TreatmentsMap returns treatments organized per date
func (ts Treatments) TreatmentsMap() TreatmentsMap {
	mk := map[string]TreatmentKey{}
	m := map[TreatmentKey]Treatments{}
	for _, t := range ts {
		k, present := mk[t.DateFormated()]
		if !present {
			k = treatmentKeyFor(t)
			mk[t.DateFormated()] = k
		}
		m[k] = append(m[k], t)
	}
	return m
}

// TreatmentEntryRow pairs a treatment with its per-time entries
// (bugs.md R5-3c, U25/D-e) — the R5-4b animal-tab accordion data source.
type TreatmentEntryRow struct {
	Treatment *Treatment
	Entries   TreatmentTimeEntries
}

// TreatmentEntriesMap organizes treatments (with their entries) per date —
// same keying as TreatmentsMap, one accordion row per expected time.
type TreatmentEntriesMap map[TreatmentKey][]TreatmentEntryRow

// OrderedKeys returns the map keys ordered by descending date (same order
// as TreatmentsMap.OrderedKeys — newest day first).
func (m TreatmentEntriesMap) OrderedKeys() []TreatmentKey {
	var keys []TreatmentKey
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Date.After(keys[j].Date)
	})
	return keys
}

// TreatmentEntriesMap organizes the treatments and their per-time entries
// per date (bugs.md R5-3c). Treatments must have their Entries association
// loaded (tx.Eager()); rows without entries still appear, with empty
// entries — the accordion renders the bitmap fallback for them.
func (ts Treatments) TreatmentEntriesMap() TreatmentEntriesMap {
	mk := map[string]TreatmentKey{}
	m := TreatmentEntriesMap{}
	for i := range ts {
		t := &ts[i]
		k, present := mk[t.DateFormated()]
		if !present {
			k = treatmentKeyFor(*t)
			mk[t.DateFormated()] = k
		}
		m[k] = append(m[k], TreatmentEntryRow{Treatment: t, Entries: t.Entries})
	}
	return m
}

// treatmentKeyFor builds the per-date accordion key (past/current/future).
func treatmentKeyFor(t Treatment) TreatmentKey {
	now := time.Now()
	nowDt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	checkDt := time.Date(t.Date.Year(), t.Date.Month(), t.Date.Day(), 0, 0, 0, 0, now.Location())
	return TreatmentKey{
		Date:    checkDt,
		DateFmt: t.DateFormated(),
		Past:    checkDt.Before(nowDt),
		Current: checkDt.Equal(nowDt),
		Future:  checkDt.After(nowDt),
	}
}

// DateFormated returns a formated date
func (t *Treatment) DateFormated() string {
	return t.Date.Format(DateFormat)
}

func nowDt() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// dt returns Date of treatment
func (t *Treatment) dt() time.Time {
	return time.Date(t.Date.Year(), t.Date.Month(), t.Date.Day(), 0, 0, 0, 0, time.Local)
}

// IsPast returns true if the treatment is in the past
func (t *Treatment) IsPast() bool {
	return t.dt().Before(nowDt())
}

// IsToday returns true if the treatment is for today
func (t *Treatment) IsToday() bool {
	return t.dt().Equal(nowDt())
}

// IsFuture returns true if the treatment is in the future
func (t *Treatment) IsFuture() bool {
	return t.dt().After(nowDt())
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (t *Treatment) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(t)
	// Wound-care treatments (issue #96) carry no medication and no posology:
	// Drug is set to the WoundCareDrugName marker and remarks hold the actual
	// content, so the posology requirement is waived.
	vs := []validate.Validator{
		&validators.TimeIsPresent{Field: t.Date, Name: "Date"},
		&validators.IntIsPresent{Field: t.AnimalID, Name: "AnimalID"},
		&validators.StringIsPresent{Field: t.Drug, Name: "Drug"},
		&validators.IntIsPresent{Field: t.Timebitmap, Name: "Timebitmap"},
	}
	if t.Drug != WoundCareDrugName {
		vs = append(vs, &validators.StringIsPresent{Field: t.Dosage, Name: "Dosage"})
	}
	return validate.Validate(vs...), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (t *Treatment) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (t *Treatment) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

func (t *Treatment) ScheduleRequired(key int) bool {
	return (t.Timebitmap & key) > 0
}

func (t *Treatment) ScheduleRequiredMorning() bool {
	return t.ScheduleRequired(Treatement_MORNING)
}

func (t *Treatment) ScheduleRequiredNoon() bool {
	return t.ScheduleRequired(Treatement_NOON)
}

func (t *Treatment) ScheduleRequiredEvening() bool {
	return t.ScheduleRequired(Treatement_EVENING)
}

func (t *Treatment) ScheduleStatus(key int) nulls.Bool {
	// bugs.md R5-3c (U25/D-e): entries are the source of truth when
	// loaded; the dormant bitmap only serves un-migrated rows.
	if s, ok := t.entryBucketStatus(key); ok {
		return s
	}
	if t.ScheduleRequired(key) {
		return nulls.NewBool((t.Timedonebitmap & key) > 0)
	}
	return nulls.Bool{Valid: false}
}

// entryBucketStatus derives the legacy 3-bucket status from the per-time
// entries (bugs.md R5-3c): a bucket with entries reports done only when
// ALL of its entries are done; buckets without entries report unknown.
// Bucket bounds mirror the legacy §10-M1 mapping: <11:00 morning,
// 11:00–15:59 noon, ≥16:00 evening.
func (t *Treatment) entryBucketStatus(key int) (nulls.Bool, bool) {
	if len(t.Entries) == 0 {
		return nulls.Bool{}, false
	}
	bounds := entryBucketBounds(key)
	if bounds == nil {
		return nulls.Bool{}, false
	}
	found, allDone := false, true
	for i := range t.Entries {
		e := &t.Entries[i]
		if !e.InBucket(bounds[0], bounds[1]) {
			continue
		}
		found = true
		if e.Status != TreatmentEntryStatusDone {
			allDone = false
		}
	}
	if !found {
		return nulls.Bool{}, false
	}
	return nulls.NewBool(allDone), true
}

// entryBucketBounds returns the [minHour, maxHour) window of a legacy
// bitmap bucket.
func entryBucketBounds(key int) []int {
	switch key {
	case Treatement_MORNING:
		return []int{0, 11}
	case Treatement_NOON:
		return []int{11, 16}
	case Treatement_EVENING:
		return []int{16, 24}
	}
	return nil
}

func (t *Treatment) ScheduleStatusMorning() nulls.Bool {
	return t.ScheduleStatus(Treatement_MORNING)
}

func (t *Treatment) ScheduleStatusNoon() nulls.Bool {
	return t.ScheduleStatus(Treatement_NOON)
}

func (t *Treatment) ScheduleStatusEvening() nulls.Bool {
	return t.ScheduleStatus(Treatement_EVENING)
}

func (t *Treatment) SetAllScheduleRequired(m bool, n bool, e bool) {
	t.Timebitmap = 0
	if m {
		t.Timebitmap |= Treatement_MORNING
	}
	if n {
		t.Timebitmap |= Treatement_NOON
	}
	if e {
		t.Timebitmap |= Treatement_EVENING
	}
}

// SetAllScheduleStatus writes the legacy Timedonebitmap.
//
// Deprecated: dormant since bugs.md R5-3 (U25/D-e) — the done state lives
// in Treatment.Entries (treatment_time_entries). Kept only for the
// rollback path; no live write path may call it.
func (t *Treatment) SetAllScheduleStatus(m bool, n bool, e bool) {
	t.Timedonebitmap = 0
	if m {
		t.Timedonebitmap |= Treatement_MORNING
	}
	if n {
		t.Timedonebitmap |= Treatement_NOON
	}
	if e {
		t.Timedonebitmap |= Treatement_EVENING
	}
}
