package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"creaves/models"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

var resyncStartMu sync.Mutex

// resyncChunkSize bounds every resync SQL statement: animals are streamed in
// keyset-paginated chunks (id > last ORDER BY id) and each chunk's
// associations are loaded with a bounded LEFT JOIN + IN(chunk ids) queries.
// Loading the whole table in one EagerPreload produced single IN() clauses
// with thousands of ids — gigantic SQL statements that MySQL parses slowly,
// that can hit max_allowed_packet, and that buffer the entire table in
// memory (bug: "gigantic SQL queries" on full resync start).
const resyncChunkSize = 200

// resyncAnimalRow is the flat scan target of the chunked LEFT JOIN query.
// Column aliases below match these db tags.
type resyncAnimalRow struct {
	ID           int          `db:"a_id"`
	Year         int          `db:"a_year"`
	YearNumber   int          `db:"a_year_number"`
	Ring         nulls.String `db:"a_ring"`
	Species      string       `db:"a_species"`
	Gender       nulls.String `db:"a_gender"`
	Cage         nulls.String `db:"a_cage"`
	Zone         nulls.String `db:"a_zone"`
	Feeding      nulls.String `db:"a_feeding"`
	ForceFeed    bool         `db:"a_force_feed"`
	AnimalageID  uuid.UUID    `db:"a_animalage_id"`
	AnimaltypeID uuid.UUID    `db:"a_animaltype_id"`
	DiscoveryID  uuid.UUID    `db:"a_discovery_id"`
	IntakeID     uuid.UUID    `db:"a_intake_id"`
	OuttakeID    nulls.UUID   `db:"a_outtake_id"`
	IntakeDate   time.Time    `db:"a_intake_date"`
	CreatedAt    time.Time    `db:"a_created_at"`
	UpdatedAt    time.Time    `db:"a_updated_at"`

	AgeID        uuid.UUID    `db:"age_id"`
	AgeName      nulls.String `db:"age_name"`
	AgeDesc      nulls.String `db:"age_description"`
	AgeDefault   nulls.Bool   `db:"age_def"`
	AgeCreatedAt nulls.Time   `db:"age_created_at"`
	AgeUpdatedAt nulls.Time   `db:"age_updated_at"`

	TypeID             uuid.UUID    `db:"type_id"`
	TypeName           nulls.String `db:"type_name"`
	TypeDefault        nulls.Bool   `db:"type_def"`
	TypeDescription    nulls.String `db:"type_description"`
	TypeHasRing        nulls.Bool   `db:"type_has_ring"`
	TypeDefaultSpecies nulls.String `db:"type_default_species"`
	TypeCreatedAt      nulls.Time   `db:"type_created_at"`
	TypeUpdatedAt      nulls.Time   `db:"type_updated_at"`

	DiscID            uuid.UUID    `db:"disc_id"`
	DiscLocation      nulls.String `db:"disc_location"`
	DiscPostalCode    nulls.String `db:"disc_postal_code"`
	DiscCity          nulls.String `db:"disc_city"`
	DiscDate          nulls.Time   `db:"disc_date"`
	DiscEntryCauseID  nulls.String `db:"disc_entry_cause_id"`
	DiscReason        nulls.String `db:"disc_reason"`
	DiscNote          nulls.String `db:"disc_note"`
	DiscDiscovererID  uuid.UUID    `db:"disc_discoverer_id"`
	DiscReturnHabitat nulls.Bool   `db:"disc_return_habitat"`
	DiscInGarden      nulls.Bool   `db:"disc_in_garden"`
	DiscCreatedAt     nulls.Time   `db:"disc_created_at"`
	DiscUpdatedAt     nulls.Time   `db:"disc_updated_at"`

	EcID         nulls.String `db:"ec_id"`
	EcCause      nulls.String `db:"ec_cause"`
	EcDetail     nulls.String `db:"ec_detail"`
	EcNature     nulls.String `db:"ec_nature"`
	EcIndication nulls.String `db:"ec_indication"`
	EcSortOrder  nulls.Int    `db:"ec_sort_order"`
	EcCreatedAt  nulls.Time   `db:"ec_created_at"`
	EcUpdatedAt  nulls.Time   `db:"ec_updated_at"`

	DiscovererID            uuid.UUID    `db:"dvr_id"`
	DiscovererFirstname     nulls.String `db:"dvr_firstname"`
	DiscovererLastname      nulls.String `db:"dvr_lastname"`
	DiscovererAddress       nulls.String `db:"dvr_address"`
	DiscovererPostalCode    nulls.String `db:"dvr_postal_code"`
	DiscovererCity          nulls.String `db:"dvr_city"`
	DiscovererCountry       nulls.String `db:"dvr_country"`
	DiscovererEmail         nulls.String `db:"dvr_email"`
	DiscovererPhone         nulls.String `db:"dvr_phone"`
	DiscovererNote          nulls.String `db:"dvr_note"`
	DiscovererReturnRequest nulls.Bool   `db:"dvr_return_request"`
	DiscovererDonation      nulls.String `db:"dvr_donation"`
	DiscovererCreatedAt     nulls.Time   `db:"dvr_created_at"`
	DiscovererUpdatedAt     nulls.Time   `db:"dvr_updated_at"`

	IntID           uuid.UUID    `db:"int_id"`
	IntDate         nulls.Time   `db:"int_date"`
	IntGeneral      nulls.String `db:"int_general"`
	IntHasWounds    nulls.Bool   `db:"int_has_wounds"`
	IntWounds       nulls.String `db:"int_wounds"`
	IntHasParasites nulls.Bool   `db:"int_has_parasites"`
	IntParasites    nulls.String `db:"int_parasites"`
	IntRemarks      nulls.String `db:"int_remarks"`
	IntCreatedAt    nulls.Time   `db:"int_created_at"`
	IntUpdatedAt    nulls.Time   `db:"int_updated_at"`

	OutID        uuid.UUID    `db:"out_id"`
	OutDate      nulls.Time   `db:"out_date"`
	OutTypeID    uuid.UUID    `db:"out_type_id"`
	OutLocation  nulls.String `db:"out_location"`
	OutNote      nulls.String `db:"out_note"`
	OutCreatedAt nulls.Time   `db:"out_created_at"`
	OutUpdatedAt nulls.Time   `db:"out_updated_at"`

	OtID             uuid.UUID    `db:"ot_id"`
	OtName           nulls.String `db:"ot_name"`
	OtDefault        nulls.Bool   `db:"ot_def"`
	OtDead           nulls.Bool   `db:"ot_dead"`
	OtError          nulls.Bool   `db:"ot_error"`
	OtRating         nulls.Int    `db:"ot_rating"`
	OtDescription    nulls.String `db:"ot_description"`
	OtDiscovererNews nulls.String `db:"ot_discoverer_news"`
	OtCreatedAt      nulls.Time   `db:"ot_created_at"`
	OtUpdatedAt      nulls.Time   `db:"ot_updated_at"`
}

// resyncChunkSelect is the bounded LEFT JOIN statement for one animal chunk.
// One row per animal: every association is 1:1 (belongs_to), so the join
// never multiplies rows. Aliases match resyncAnimalRow's db tags exactly —
// pop strict-maps raw query columns, and animals.IntakeDate (the legacy
// mixed-case column) is aliased to lowercase so the scan target is identical
// on MySQL and SQLite (column names are case-insensitive in both dialects).
const resyncChunkSelect = `SELECT
  a.id AS a_id, a.year AS a_year, a.yearNumber AS a_year_number, a.ring AS a_ring,
  a.species AS a_species, a.gender AS a_gender, a.cage AS a_cage, a.zone AS a_zone,
  a.feeding AS a_feeding, a.force_feed AS a_force_feed,
  a.animalage_id AS a_animalage_id, a.animaltype_id AS a_animaltype_id,
  a.discovery_id AS a_discovery_id, a.intake_id AS a_intake_id, a.outtake_id AS a_outtake_id,
  a.IntakeDate AS a_intake_date, a.created_at AS a_created_at, a.updated_at AS a_updated_at,
  age.id AS age_id, age.name AS age_name, age.description AS age_description, age.def AS age_def,
  age.created_at AS age_created_at, age.updated_at AS age_updated_at,
  atype.id AS type_id, atype.name AS type_name, atype.def AS type_def, atype.description AS type_description,
  atype.has_ring AS type_has_ring, atype.default_species AS type_default_species,
  atype.created_at AS type_created_at, atype.updated_at AS type_updated_at,
  d.id AS disc_id, d.location AS disc_location, d.postal_code AS disc_postal_code, d.city AS disc_city,
  d.date AS disc_date, d.entry_cause_id AS disc_entry_cause_id, d.reason AS disc_reason, d.note AS disc_note,
  d.discoverer_id AS disc_discoverer_id, d.return_habitat AS disc_return_habitat, d.in_garden AS disc_in_garden,
  d.created_at AS disc_created_at, d.updated_at AS disc_updated_at,
  ec.id AS ec_id, ec.cause AS ec_cause, ec.detail AS ec_detail, ec.nature AS ec_nature,
  ec.indication AS ec_indication, ec.sort_order AS ec_sort_order,
  ec.created_at AS ec_created_at, ec.updated_at AS ec_updated_at,
  dvr.id AS dvr_id, dvr.firstname AS dvr_firstname, dvr.lastname AS dvr_lastname,
  dvr.address AS dvr_address, dvr.postal_code AS dvr_postal_code, dvr.city AS dvr_city,
  dvr.country AS dvr_country, dvr.email AS dvr_email, dvr.phone AS dvr_phone, dvr.note AS dvr_note,
  dvr.return_request AS dvr_return_request, dvr.donation AS dvr_donation,
  dvr.created_at AS dvr_created_at, dvr.updated_at AS dvr_updated_at,
  i.id AS int_id, i.date AS int_date, i.general AS int_general, i.has_wounds AS int_has_wounds,
  i.wounds AS int_wounds, i.has_parasites AS int_has_parasites, i.parasites AS int_parasites,
  i.remarks AS int_remarks, i.created_at AS int_created_at, i.updated_at AS int_updated_at,
  o.id AS out_id, o.date AS out_date, o.outtaketype_id AS out_type_id, o.location AS out_location,
  o.note AS out_note, o.created_at AS out_created_at, o.updated_at AS out_updated_at,
  ot.id AS ot_id, ot.name AS ot_name, ot.def AS ot_def, ot.dead AS ot_dead, ot.error AS ot_error,
  ot.rating AS ot_rating, ot.description AS ot_description, ot.discoverer_news AS ot_discoverer_news,
  ot.created_at AS ot_created_at, ot.updated_at AS ot_updated_at
FROM animals a
LEFT JOIN animalages age ON age.id = a.animalage_id
LEFT JOIN animaltypes atype ON atype.id = a.animaltype_id
LEFT JOIN discoveries d ON d.id = a.discovery_id
LEFT JOIN entry_causes ec ON ec.id = d.entry_cause_id
LEFT JOIN discoverers dvr ON dvr.id = d.discoverer_id
LEFT JOIN intakes i ON i.id = a.intake_id
LEFT JOIN outtakes o ON o.id = a.outtake_id
LEFT JOIN outtaketypes ot ON ot.id = o.outtaketype_id
WHERE a.id > ?
ORDER BY a.id
LIMIT ?`

// countAnimals returns the total number of animals — StartResync's cheap
// replacement for loading the whole table just to take len().
func countAnimals(tx *pop.Connection) (int, error) {
	row := struct {
		Total int `db:"total"`
	}{}
	if err := tx.RawQuery("SELECT COUNT(*) AS total FROM animals").First(&row); err != nil {
		return 0, err
	}
	return row.Total, nil
}

// loadResyncAnimalChunk returns up to resyncChunkSize animals with id >
// afterID, fully populated (same association set as the old EagerPreload
// list), plus the id cursor for the next chunk.
func loadResyncAnimalChunk(tx *pop.Connection, afterID int) (*models.Animals, int, error) {
	rows := []resyncAnimalRow{}
	if err := tx.RawQuery(resyncChunkSelect, afterID, resyncChunkSize).All(&rows); err != nil {
		return nil, afterID, err
	}
	animals := models.Animals{}
	for i := range rows {
		animals = append(animals, *resyncRowToAnimal(&rows[i]))
	}
	if len(rows) > 0 {
		afterID = rows[len(rows)-1].ID
	}
	return &animals, afterID, nil
}

// resyncRowToAnimal rebuilds the models.Animal graph one LEFT JOIN row
// represents. Zero uuid ids mean the LEFT JOIN found no row — the association
// stays zero, exactly like an unloaded pop belongs_to.
func resyncRowToAnimal(r *resyncAnimalRow) *models.Animal {
	a := &models.Animal{
		ID:           r.ID,
		Year:         r.Year,
		YearNumber:   r.YearNumber,
		Ring:         r.Ring,
		Species:      r.Species,
		Gender:       r.Gender,
		Cage:         r.Cage,
		Zone:         r.Zone,
		Feeding:      r.Feeding,
		ForceFeed:    r.ForceFeed,
		AnimalageID:  r.AnimalageID,
		AnimaltypeID: r.AnimaltypeID,
		DiscoveryID:  r.DiscoveryID,
		IntakeID:     r.IntakeID,
		OuttakeID:    r.OuttakeID,
		IntakeDate:   r.IntakeDate,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if r.AgeID != uuid.Nil {
		a.Animalage = models.Animalage{
			ID:          r.AgeID,
			Name:        r.AgeName.String,
			Description: r.AgeDesc,
			Default:     r.AgeDefault.Bool,
			CreatedAt:   r.AgeCreatedAt.Time,
			UpdatedAt:   r.AgeUpdatedAt.Time,
		}
	}
	if r.TypeID != uuid.Nil {
		a.Animaltype = models.Animaltype{
			ID:             r.TypeID,
			Name:           r.TypeName.String,
			Default:        r.TypeDefault.Bool,
			Description:    r.TypeDescription,
			HasRing:        r.TypeHasRing.Bool,
			DefaultSpecies: r.TypeDefaultSpecies,
			CreatedAt:      r.TypeCreatedAt.Time,
			UpdatedAt:      r.TypeUpdatedAt.Time,
		}
	}
	if r.DiscID != uuid.Nil {
		a.Discovery = models.Discovery{
			ID:            r.DiscID,
			Location:      r.DiscLocation,
			PostalCode:    r.DiscPostalCode,
			City:          r.DiscCity,
			Date:          r.DiscDate.Time,
			EntryCauseID:  r.DiscEntryCauseID.String,
			Reason:        r.DiscReason,
			Note:          r.DiscNote,
			DiscovererID:  r.DiscDiscovererID,
			ReturnHabitat: r.DiscReturnHabitat.Bool,
			InGarden:      r.DiscInGarden.Bool,
			CreatedAt:     r.DiscCreatedAt.Time,
			UpdatedAt:     r.DiscUpdatedAt.Time,
		}
		if r.EcID.Valid && r.EcID.String != "" {
			a.Discovery.EntryCause = models.EntryCause{
				ID:         r.EcID.String,
				Cause:      r.EcCause.String,
				Detail:     r.EcDetail.String,
				Nature:     r.EcNature.String,
				Indication: r.EcIndication.String,
				SortOrder:  r.EcSortOrder.Int,
				CreatedAt:  r.EcCreatedAt.Time,
				UpdatedAt:  r.EcUpdatedAt.Time,
			}
		}
		if r.DiscovererID != uuid.Nil {
			a.Discovery.Discoverer = models.Discoverer{
				ID:            r.DiscovererID,
				Firstname:     r.DiscovererFirstname,
				Lastname:      r.DiscovererLastname,
				Address:       r.DiscovererAddress,
				PostalCode:    r.DiscovererPostalCode,
				City:          r.DiscovererCity,
				Country:       r.DiscovererCountry,
				Email:         r.DiscovererEmail,
				Phone:         r.DiscovererPhone,
				Note:          r.DiscovererNote,
				ReturnRequest: r.DiscovererReturnRequest.Bool,
				Donation:      r.DiscovererDonation,
				CreatedAt:     r.DiscovererCreatedAt.Time,
				UpdatedAt:     r.DiscovererUpdatedAt.Time,
			}
		}
	}
	if r.IntID != uuid.Nil {
		a.Intake = models.Intake{
			ID:           r.IntID,
			Date:         r.IntDate.Time,
			General:      r.IntGeneral,
			HasWounds:    r.IntHasWounds.Bool,
			Wounds:       r.IntWounds,
			HasParasites: r.IntHasParasites.Bool,
			Parasites:    r.IntParasites,
			Remarks:      r.IntRemarks,
			CreatedAt:    r.IntCreatedAt.Time,
			UpdatedAt:    r.IntUpdatedAt.Time,
		}
	}
	if r.OuttakeID.Valid {
		a.Outtake = &models.Outtake{
			ID:        r.OutID,
			Date:      r.OutDate.Time,
			TypeID:    r.OutTypeID,
			Location:  r.OutLocation,
			Note:      r.OutNote,
			CreatedAt: r.OutCreatedAt.Time,
			UpdatedAt: r.OutUpdatedAt.Time,
		}
		if r.OtID != uuid.Nil {
			a.Outtake.Type = models.Outtaketype{
				ID:             r.OtID,
				Name:           r.OtName.String,
				Default:        r.OtDefault.Bool,
				Dead:           r.OtDead.Bool,
				Error:          r.OtError.Bool,
				Rating:         r.OtRating.Int,
				Description:    r.OtDescription,
				DiscovererNews: r.OtDiscovererNews,
				CreatedAt:      r.OtCreatedAt.Time,
				UpdatedAt:      r.OtUpdatedAt.Time,
			}
		}
	}
	return a
}

// ErrWebhookDisabled is returned by StartResync when webhook forwarding is
// off. Handlers must map it to a clear user-facing message (flash + 200
// with an enable-on-confirm offer) instead of the raw buffalo error trace.
var ErrWebhookDisabled = fmt.Errorf("webhook forwarding is disabled: enable webhook forwarding to run resync")

// StartResync creates one run and schedules its work on a background goroutine.
// With force=true, state events that already exist (same instance/animal/
// content hash) are re-queued for delivery instead of being skipped: this is
// the "full rebuild" path used after the console side purged the instance
// (cleanup). Re-delivered events keep their deterministic UUIDs, so the
// console upsert/dedup keeps the operation idempotent.
func StartResync(tx *pop.Connection, instanceID string, total int, force bool) (*models.ResyncRun, error) {
	resyncStartMu.Lock()
	defer resyncStartMu.Unlock()
	if !IsWebhookEnabled() {
		return nil, ErrWebhookDisabled
	}
	active, err := tx.Where("instance_id = ? AND status = ?", instanceID, "running").Exists(&models.ResyncRun{})
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("resync already running")
	}
	if total == 0 {
		// COUNT(*) only — the old code loaded the whole animals table here
		// just to take len(), the first "gigantic query" of a full resync.
		var err error
		total, err = countAnimals(tx)
		if err != nil {
			return nil, err
		}
	}
	now := time.Now()
	run := &models.ResyncRun{ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID, Status: "running", StartedAt: now, TotalAnimals: total}
	// The worker goroutine below reads this row back through models.DB on a
	// DIFFERENT connection. Creating it on the caller's tx would race the
	// request transaction's commit (the goroutine's Find can execute before
	// the row is visible -> "no rows" -> worker exits and the run stays
	// 'running' forever). Persist on models.DB (autocommit) so the row is
	// committed before the goroutine is launched.
	createTx := models.DB
	if createTx == nil {
		createTx = tx
	}
	if err := createTx.Create(run); err != nil {
		return nil, err
	}
	// A resync creates events asynchronously; ensure delivery is available and
	// wake it immediately rather than waiting for the fallback poll interval.
	EnsureWebhookWorkerRunning()
	signalWebhookWake()
	// The worker shares models.DB like any other code path. NOTE: with
	// pop v6.1.0 + pop.Debug (development), every Create/Update leaked one
	// pooled connection via the SQL logger (logger.go store.Transaction()
	// without close) — resync loops then wedged the app (pool deadlock) or
	// exhausted MySQL (Error 1040). Worked around by the SetTxLogger
	// override in models/models.go (fixed upstream in pop v6.1.2); dev
	// pool limits in database.yml guard against any future burst.
	workerTx := models.DB
	if workerTx == nil {
		workerTx = tx
	}
	go func() {
		if err := RunResync(context.Background(), workerTx, run.ID, force); err != nil {
			log.Printf("resync run %s failed: %v", run.ID, err)
		}
	}()
	return run, nil
}

// resyncPersistEvery controls how often the resync loop persists run progress
// and re-checks for user cancellation. Per-animal persistence used to issue an
// UPDATE plus a run re-read SELECT per animal — log noise and avoidable
// round-trips; 25 gives imperceptible status.json granularity at resync scale.
const resyncPersistEvery = 25

// resyncStateIndex is the run-wide set of (animal_id, content_hash) state
// events already in event_streams for one instance. One query replaces the
// per-animal EXISTS check; entries created by the run itself are added in
// memory. animal_state events are also created by the update path
// (PublishAnimalStateEvent); both paths share the deterministic
// (instance, animal, hash) identity, and PublishAnimalStateEvent tolerates
// losing the insert race to a concurrent resync.
type resyncStateIndex struct {
	entries map[int]map[string]bool
}

func loadResyncStateIndex(tx *pop.Connection, instanceID string) (*resyncStateIndex, error) {
	rows := []struct {
		AnimalID    int    `db:"animal_id"`
		ContentHash string `db:"content_hash"`
	}{}
	if err := tx.RawQuery(
		"SELECT animal_id, content_hash FROM event_streams WHERE instance_id = ? AND event_type = ? AND content_hash IS NOT NULL",
		instanceID, string(models.EventTypeAnimalState),
	).All(&rows); err != nil {
		return nil, err
	}
	ix := &resyncStateIndex{entries: map[int]map[string]bool{}}
	for i := range rows {
		ix.add(rows[i].AnimalID, rows[i].ContentHash)
	}
	return ix, nil
}

func (ix *resyncStateIndex) has(animalID int, hash string) bool {
	return ix.entries[animalID][hash]
}

func (ix *resyncStateIndex) add(animalID int, hash string) {
	if ix.entries[animalID] == nil {
		ix.entries[animalID] = map[string]bool{}
	}
	ix.entries[animalID][hash] = true
}

// resyncCheckpoint persists run progress and re-checks for user cancellation
// every resyncPersistEvery animals; context cancellation is honoured on every
// call. stop=true means the loop must return immediately.
func resyncCheckpoint(ctx context.Context, tx *pop.Connection, run *models.ResyncRun, processed int) (stop bool, err error) {
	if err := ctx.Err(); err != nil {
		run.Cancel(time.Now())
		_ = tx.Update(run)
		return true, err
	}
	if processed%resyncPersistEvery != 0 {
		return false, nil
	}
	if err := tx.Update(run); err != nil {
		return true, err
	}
	cancelled, err := tx.Where("id = ? AND status = ?", run.ID, "cancelled").Exists(&models.ResyncRun{})
	if err != nil {
		return true, err
	}
	return cancelled, nil
}

// applyCurrentStatus sets payload.CurrentStatus the way every producer of a
// full-state event must before hashing: "in_care", or "released" when the
// animal has an outtake. The resync, the update path and the sync-status
// computation all call this so their StateContentHashPayload hashes stay
// comparable — a status-less hash never matches any stored content_hash and
// every animal would show as unconfirmed forever.
func applyCurrentStatus(payload *models.EventPayload, animal *models.Animal) {
	payload.CurrentStatus = "in_care"
	if animal.Outtake != nil {
		payload.CurrentStatus = "released"
	}
}

// processResyncAnimal builds and enqueues the state event for one animal,
// incrementing the processed counter. Progress persistence is the caller's
// (resyncCheckpoint) job.
func processResyncAnimal(tx *pop.Connection, run *models.ResyncRun, pre *translationPreloader, ix *resyncStateIndex, force bool, animal *models.Animal) error {
	payload := buildEventPayloadInto(tx, pre, animal)
	if payload == nil {
		appendResyncError(run, animal.ID, "failed to build payload")
	} else {
		applyCurrentStatus(payload, animal)
		if err := enqueueResyncStateEvent(tx, run, ix, force, animal.ID, *payload); err != nil {
			return err
		}
	}
	run.AnimalsProcessed++
	return nil
}

func RunResync(ctx context.Context, tx *pop.Connection, runID uuid.UUID, force bool) error {
	run := &models.ResyncRun{}
	if err := tx.Find(run, runID); err != nil {
		// Never leave a run stranded in 'running': mark it failed
		// best-effort (the row may be invisible only to this connection).
		_ = tx.RawQuery(
			"UPDATE resync_runs SET status = 'failed', finished_at = ?, errors = ? WHERE id = ? AND status = 'running'",
			time.Now(), fmt.Sprintf("worker start: %v", err), runID,
		).Exec()
		return err
	}
	// Stream the animals in keyset-paginated chunks. The old code ran a
	// single EagerPreload over the WHOLE table: one SELECT per association
	// with an IN() clause of thousands of ids (the "gigantic SQL queries"
	// bug), plus the full table buffered in memory. Each chunk now loads its
	// nested associations with ONE bounded LEFT JOIN (all associations are
	// 1:1 belongs_to, so no row multiplication); the per-chunk translation
	// preloader then issues bounded IN(chunk) queries only.
	ix, err := loadResyncStateIndex(tx, run.InstanceID)
	if err != nil {
		return finishResync(tx, run, err)
	}
	var hashLines []stateHashLine
	processed := 0
	afterID := 0
	for {
		done, err := processResyncChunk(ctx, tx, run, ix, force, &processed, &afterID, &hashLines)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	// Persist the announcement once: totals/checksum cover every animal of
	// the run.
	if hashLines != nil {
		announceLines := make([]string, 0, len(hashLines))
		for _, hl := range hashLines {
			announceLines = append(announceLines, fmt.Sprintf("%d|%s", hl.AnimalID, hl.Hash))
		}
		now := time.Now()
		checksum := StateSetChecksum(announceLines)
		run.AnnouncedExpectedTotal = len(announceLines)
		run.AnnouncedExpectedChecksum = &checksum
		run.AnnouncedAt = &now
		if err := tx.Update(run); err != nil {
			return finishResync(tx, run, err)
		}
	}
	// Production is only half the job: the run may only report "completed"
	// once every created event was accepted by the console (bug #1: the old
	// code marked the run completed right after enqueuing, so partial
	// webhook accepts silently lost events while the run looked green).
	return completeResyncDelivery(ctx, tx, run)
}

// processResyncChunk handles one keyset-paginated chunk: load the animals
// with their associations (one bounded LEFT JOIN), build the per-chunk
// translation preloader (bounded IN(chunk) queries), accumulate the expected
// state-hash lines for the run announcement, and enqueue the state event of
// every animal in the chunk. done=true means the run must stop — either all
// animals were processed or the run was cancelled; err is terminal.
func processResyncChunk(ctx context.Context, tx *pop.Connection, run *models.ResyncRun, ix *resyncStateIndex, force bool, processed, afterID *int, hashLines *[]stateHashLine) (bool, error) {
	animals, nextID, err := loadResyncAnimalChunk(tx, *afterID)
	if err != nil {
		return true, finishResync(tx, run, err)
	}
	if len(*animals) == 0 {
		return true, nil
	}
	*afterID = nextID
	pre := newTranslationPreloader(tx, animals)
	// Announce the expected sync state for this run (checksum bug fix):
	// hashes are computed per chunk from the same already-loaded
	// associations the resync loop uses — ZERO extra SELECTs. Persisted
	// once all chunks contributed, and echoed in every delivery envelope
	// of this run ("sync" block).
	*hashLines = append(*hashLines, expectedStateHashes(tx, animals, pre, run.InstanceID)...)
	for i := range *animals {
		stop, err := resyncCheckpoint(ctx, tx, run, *processed)
		if err != nil {
			return true, err
		}
		if stop {
			return true, nil
		}
		if err := processResyncAnimal(tx, run, pre, ix, force, &(*animals)[i]); err != nil {
			return true, finishResync(tx, run, err)
		}
		*processed++
		// Keep delivery moving while large resyncs are still producing events.
		signalWebhookWake()
	}
	return false, nil
}

// resyncDeliveryPollDelay and resyncDeliveryMaxStalled bound the delivery
// wait: after MaxStalled consecutive attempts without delivery progress the
// run is marked failed with a per-run diagnostic instead of blocking forever.
// Package vars so tests can tighten them.
var (
	resyncDeliveryPollDelay  = 100 * time.Millisecond
	resyncDeliveryMaxStalled = 5
)

// countResyncRunEvents returns (total, delivered) for the events attributed
// to this run (created or re-queued by it, see enqueueResyncStateEvent).
func countResyncRunEvents(tx *pop.Connection, runID uuid.UUID) (int, int, error) {
	row := struct {
		Total     int `db:"total"`
		Delivered int `db:"delivered"`
	}{}
	if err := tx.RawQuery(
		"SELECT COUNT(*) AS total, "+
			"COALESCE(SUM(CASE WHEN delivered_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS delivered "+
			"FROM event_streams WHERE resync_run_id = ?",
		runID,
	).First(&row); err != nil {
		return 0, 0, err
	}
	return row.Total, row.Delivered, nil
}

// failResyncDelivery marks the run failed and appends the diagnostic to the
// run's structured error list without discarding production errors.
func failResyncDelivery(tx *pop.Connection, run *models.ResyncRun, diagnostic string) error {
	now := time.Now()
	run.Status = "failed"
	run.FinishedAt = &now
	appendResyncError(run, 0, diagnostic)
	return tx.Update(run)
}

// resyncDeliveryCancelled reports whether delivery must stop (context done
// or user cancellation) and persists the cancelled state when so.
func resyncDeliveryCancelled(ctx context.Context, tx *pop.Connection, run *models.ResyncRun) (bool, error) {
	if err := ctx.Err(); err != nil {
		run.Cancel(time.Now())
		_ = tx.Update(run)
		return true, nil
	}
	cancelled, err := tx.Where("id = ? AND status = ?", run.ID, "cancelled").Exists(&models.ResyncRun{})
	if err != nil {
		return false, err
	}
	if cancelled {
		run.Cancel(time.Now())
		_ = tx.Update(run)
	}
	return cancelled, nil
}

// resyncDeliveryPump drives one delivery attempt and re-counts the run's
// delivered events, returning the updated stall counter (0 on progress).
func resyncDeliveryPump(tx *pop.Connection, run *models.ResyncRun, deliveredBefore, stalled int) (int, error) {
	// The background worker shares this duty; driving deliverBatch here keeps
	// the run responsive instead of waiting for the 60s fallback tick.
	// Deliveries are idempotent (console-side upsert), so overlap with the
	// worker is harmless.
	if _, err := deliverBatch(); err != nil {
		log.Printf("resync run %s delivery attempt failed: %v", run.ID, err)
	}
	_, deliveredNow, err := countResyncRunEvents(tx, run.ID)
	if err != nil {
		return stalled, err
	}
	if deliveredNow > deliveredBefore {
		return 0, nil
	}
	return stalled + 1, nil
}

// resyncDeliveryFinished records the delivered/failed counters and reports
// whether the run is done (nothing created or everything delivered), marking
// it completed when so.
func resyncDeliveryFinished(tx *pop.Connection, run *models.ResyncRun, total, delivered int) (bool, error) {
	run.EventsDelivered = delivered
	run.EventsFailed = total - delivered
	if total == 0 || delivered == total {
		run.Complete(time.Now())
		return true, tx.Update(run)
	}
	return false, nil
}

// completeResyncDelivery drives the webhook deliverer until every event of
// the run is accepted by the console (run -> completed) or a bounded number
// of stalled attempts proves delivery impossible (run -> failed with
// diagnostics counting delivered vs failed events). Partial batch accepts
// (e.g. 97/100) are retried automatically: rejected events stay
// delivered_at IS NULL and the next deliverBatch picks them up again.
func completeResyncDelivery(ctx context.Context, tx *pop.Connection, run *models.ResyncRun) error {
	rr := &resyncDeliveryRunner{tx: tx, run: run}
	for {
		done, err := rr.iterate(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(resyncDeliveryPollDelay)
	}
}

// resyncDeliveryRunner carries the mutable state of one run's delivery wait
// (the stall counter) across loop iterations.
type resyncDeliveryRunner struct {
	tx      *pop.Connection
	run     *models.ResyncRun
	stalled int
}

// iterate performs one delivery-wait step (re-check cancellation, count,
// complete, persist progress, pump one batch, enforce the stall bound).
// done=true means the loop must stop; err is a terminal error for the caller.
func (r *resyncDeliveryRunner) iterate(ctx context.Context) (bool, error) {
	cancelled, err := resyncDeliveryCancelled(ctx, r.tx, r.run)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if cancelled {
		return true, nil
	}
	total, delivered, err := countResyncRunEvents(r.tx, r.run.ID)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	finished, err := resyncDeliveryFinished(r.tx, r.run, total, delivered)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if finished {
		return true, nil
	}
	// Persist the live delivered/failed counters so the status.json endpoint
	// and the resync UI show delivery progress.
	if err := r.tx.Update(r.run); err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	r.stalled, err = resyncDeliveryPump(r.tx, r.run, delivered, r.stalled)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if r.stalled >= resyncDeliveryMaxStalled {
		return true, failResyncDelivery(r.tx, r.run, fmt.Sprintf(
			"delivery incomplete: %d of %d events accepted; %d events not delivered after %d stalled attempts",
			delivered, total, total-delivered, r.stalled))
	}
	return false, nil
}

func appendResyncError(run *models.ResyncRun, animalID int, message string) {
	var errors []map[string]interface{}
	if run.Errors != "" {
		_ = json.Unmarshal([]byte(run.Errors), &errors)
	}
	errors = append(errors, map[string]interface{}{"animal_id": animalID, "error": message})
	data, _ := json.Marshal(errors)
	run.Errors = string(data)
}

// enqueueResyncStateEvent creates (or, in force mode, re-queues) the
// deterministic full-state event for one animal.
//   - Unknown (instance, animal, hash): create the event.
//   - Known, force=false: counted as skipped unchanged.
//   - Known, force=true: re-queue by resetting delivered_at so the webhook
//     worker delivers it again; the deterministic UUID keeps the console
//     side idempotent.
func enqueueResyncStateEvent(tx *pop.Connection, run *models.ResyncRun, ix *resyncStateIndex, force bool, animalID int, payload models.EventPayload) error {
	hash := StateContentHashPayload(run.InstanceID, payload)
	// The console's no-op dedupe compares payload.state_hash against the
	// stored snapshot; without it every delivery would re-apply. Same hash as
	// the column and as the update-path state events.
	payload.StateHash = hash
	exists := ix.has(animalID, hash)
	if exists && !force {
		run.EventsSkippedUnchanged++
		return nil
	}
	if exists {
		// Force re-queues may be YEARS after the event was created; the stored
		// payload can predate the state_hash field (without it the console
		// cannot acknowledge the delivery). Adopt the freshly built payload —
		// same canonical state plus state_hash — so every re-queued event is
		// acknowledgeable.
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if err := tx.RawQuery(
			// resync_run_id is re-pointed at the current run so the
			// delivery-accounting loop below sees the re-queued event as
			// this run's responsibility (created/failed counting).
			// acknowledged_at is reset too: the previous acknowledgement
			// belonged to the earlier delivery; the fresh confirmation
			// must come from the console again.
			"UPDATE event_streams SET delivered_at = NULL, acknowledged_at = NULL, payload = ?, resync_run_id = ? WHERE instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?",
			[]byte(data), run.ID, run.InstanceID, animalID, string(models.EventTypeAnimalState), hash,
		).Exec(); err != nil {
			return err
		}
		run.EventsCreated++
		return nil
	}
	id := StateEventUUID(run.InstanceID, animalID, hash)
	event := &models.EventStream{ID: id, InstanceID: run.InstanceID, AnimalID: animalID, EventType: string(models.EventTypeAnimalState), ContentHash: &hash, ResyncRunID: &run.ID}
	if err := event.SetPayload(payload); err != nil {
		appendResyncError(run, animalID, err.Error())
		return nil
	}
	if err := tx.Create(event); err != nil {
		return err
	}
	ix.add(animalID, hash)
	run.EventsCreated++
	return nil
}

func finishResync(tx *pop.Connection, run *models.ResyncRun, err error) error {
	run.Fail(time.Now(), err)
	_ = tx.Update(run)
	return err
}

func RecoverInterruptedRuns(tx *pop.Connection) error {
	runs := &[]models.ResyncRun{}
	if err := tx.Where("status = ?", "running").All(runs); err != nil {
		return err
	}
	for i := range *runs {
		r := &(*runs)[i]
		r.Fail(time.Now(), fmt.Errorf("interrupted by restart"))
		if err := tx.Update(r); err != nil {
			return err
		}
	}
	return nil
}

func CancelResync(tx *pop.Connection, runID uuid.UUID) error {
	run := &models.ResyncRun{}
	if err := tx.Find(run, runID); err != nil {
		return err
	}
	if run.Status == "running" {
		run.Cancel(time.Now())
		return tx.Update(run)
	}
	return nil
}
