package actions

import (
	"context"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Regression test: RunResync must emit full-state events whose payload
// contains nested association data (outtake type/rating/dead, entry-cause
// fields). Plain tx.Eager() loads only direct associations, so events used to
// go out with empty outtake.type and entry_cause — the console then stored
// NULLs and annual-report buckets silently lost the data.

func TestRunResyncEmitsNestedOuttakeAndEntryCause(t *testing.T) {
	now := "NOW()"

	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
			t.Fatalf("fixture insert failed: %v\nquery: %s", err, q)
		}
	}

	// Pre-clean residue from a previous failed run (t.Cleanup is registered
	// after the inserts, so a Fatalf mid-setup would leave rows behind).
	for _, q := range []string{
		"DELETE FROM event_streams WHERE animal_id = 985049",
		"DELETE FROM resync_runs WHERE instance_id = 'rsync-resync-test'",
		"DELETE FROM animals WHERE id = 985049",
		"DELETE FROM outtakes WHERE id = '77777777-0000-0000-0000-0000000000f0'",
		"DELETE FROM discoveries WHERE id = '66666666-0000-0000-0000-0000000000f0'",
		"DELETE FROM intakes WHERE id = '55555555-0000-0000-0000-0000000000f0'",
		"DELETE FROM discoverers WHERE id = 'dddddddd-4444-4444-4444-4444444444f1'",
		"DELETE FROM entry_causes WHERE id = 'RSYNC_EC1'",
		"DELETE FROM outtaketypes WHERE id = 'cccccccc-3333-3333-3333-3333333333f1'",
		"DELETE FROM animaltypes WHERE id = 'bbbbbbbb-2222-2222-2222-2222222222f1'",
		"DELETE FROM animalages WHERE id = 'aaaaaaaa-1111-1111-1111-1111111111f1'",
	} {
		if err := models.DB.RawQuery(q).Exec(); err != nil {
			t.Logf("pre-clean failed: %v (%s)", err, q)
		}
	}

	exec("INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('aaaaaaaa-1111-1111-1111-1111111111f1', 'RSYNC Young', 0, " + now + ", " + now + ")")
	exec("INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('bbbbbbbb-2222-2222-2222-2222222222f1', 'RSYNC Type', 0, " + now + ", " + now + ")")
	exec("INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('cccccccc-3333-3333-3333-3333333333f1', 'RSYNC_REL', 0, " + now + ", " + now + ", 0, 1, 0)")
	exec("INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('RSYNC_EC1', 'RSYNC_C1', 'RSYNC_D1', 'RSYNC_N1', 'x', " + now + ", " + now + ", 1)")
	exec("INSERT INTO discoverers (id, created_at, updated_at) VALUES ('dddddddd-4444-4444-4444-4444444444f1', " + now + ", " + now + ")")

	exec("INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('55555555-0000-0000-0000-0000000000f0', '1998-06-01 10:00:00', " + now + ", " + now + ")")
	exec("INSERT INTO discoveries (id, date, discoverer_id, entry_cause_id, created_at, updated_at) VALUES ('66666666-0000-0000-0000-0000000000f0', '1998-06-01 10:00:00', 'dddddddd-4444-4444-4444-4444444444f1', 'RSYNC_EC1', " + now + ", " + now + ")")
	exec("INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('77777777-0000-0000-0000-0000000000f0', '1998-06-01 10:00:00', 'cccccccc-3333-3333-3333-3333333333f1', " + now + ", " + now + ")")
	exec("INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, outtake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (985049, 'RSYNC_Hedgehog', 'aaaaaaaa-1111-1111-1111-1111111111f1', 'bbbbbbbb-2222-2222-2222-2222222222f1', '66666666-0000-0000-0000-0000000000f0', '55555555-0000-0000-0000-0000000000f0', '77777777-0000-0000-0000-0000000000f0', " + now + ", " + now + ", 1998, 50, '1998-06-01 10:00:00')")

	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM event_streams WHERE animal_id = 985049",
			"DELETE FROM resync_runs WHERE instance_id = 'rsync-resync-test'",
			"DELETE FROM animals WHERE id = 985049",
			"DELETE FROM outtakes WHERE id = '77777777-0000-0000-0000-0000000000f0'",
			"DELETE FROM discoveries WHERE id = '66666666-0000-0000-0000-0000000000f0'",
			"DELETE FROM intakes WHERE id = '55555555-0000-0000-0000-0000000000f0'",
			"DELETE FROM discoverers WHERE id = 'dddddddd-4444-4444-4444-4444444444f1'",
			"DELETE FROM entry_causes WHERE id = 'RSYNC_EC1'",
			"DELETE FROM outtaketypes WHERE id = 'cccccccc-3333-3333-3333-3333333333f1'",
			"DELETE FROM animaltypes WHERE id = 'bbbbbbbb-2222-2222-2222-2222222222f1'",
			"DELETE FROM animalages WHERE id = 'aaaaaaaa-1111-1111-1111-1111111111f1'",
		} {
			if err := models.DB.RawQuery(q).Exec(); err != nil {
				t.Logf("teardown failed: %v (%s)", err, q)
			}
		}
	})

	run := &models.ResyncRun{
		ID:           uuid.Must(uuid.NewV4()),
		InstanceID:   "rsync-resync-test",
		Status:       "running",
		StartedAt:    time.Now(),
		TotalAnimals: 1,
	}
	if err := models.DB.Create(run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := RunResync(context.Background(), models.DB, run.ID); err != nil {
		t.Fatalf("RunResync: %v", err)
	}

	ev := &models.EventStream{}
	if err := models.DB.Where("animal_id = ? AND instance_id = ?", 985049, "rsync-resync-test").First(ev); err != nil {
		t.Fatalf("no event emitted: %v", err)
	}
	payload, err := ev.GetPayload()
	if err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.Animal.AnimalAge != "RSYNC Young" {
		t.Errorf("animal.animal_age = %q, want RSYNC Young (direct association dropped by explicit Eager list)", payload.Animal.AnimalAge)
	}
	if payload.Animal.AnimalType != "RSYNC Type" {
		t.Errorf("animal.animal_type = %q, want RSYNC Type", payload.Animal.AnimalType)
	}
	if payload.Outtake.Type != "RSYNC_REL" {
		t.Errorf("outtake.type = %q, want RSYNC_REL (nested Outtake.Type not loaded)", payload.Outtake.Type)
	}
	if payload.Outtake.Rating != 1 {
		t.Errorf("outtake.rating = %d, want 1", payload.Outtake.Rating)
	}
	if payload.Discovery.EntryCause == "" {
		t.Errorf("discovery.entry_cause empty (nested Discovery.EntryCause not loaded)")
	}
	if payload.Discovery.EntryCauseDetail != "RSYNC_D1" {
		t.Errorf("discovery.entry_cause_detail = %q, want RSYNC_D1", payload.Discovery.EntryCauseDetail)
	}
	if payload.Discovery.EntryCauseNature != "RSYNC_N1" {
		t.Errorf("discovery.entry_cause_nature = %q, want RSYNC_N1", payload.Discovery.EntryCauseNature)
	}
}

// Regression test: StartResync must leave the run row COMMITTED before
// returning. The handler passes the request transaction, whose commit only
// happens after the handler returns — but the background worker reads the
// row back through models.DB on a different connection. When the Create went
// through the request tx, the worker's Find could run before the commit,
// get "no rows", and exit silently — leaving the run stranded in 'running'
// forever (observed in e2e: run stuck at animals_processed=0, no error).
func TestStartResyncRunCommittedBeforeReturn(t *testing.T) {
	saved := CurrentConfig
	CurrentConfig = &models.Config{InstanceID: "rsync-commit-test"}
	if err := CurrentConfig.SetSettings(models.ConfigSettings{WebhookEnabled: true, WebhookURL: "http://127.0.0.1:1/unreachable"}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	t.Cleanup(func() {
		CurrentConfig = saved
		if err := models.DB.RawQuery("DELETE FROM resync_runs WHERE instance_id = 'rsync-commit-test'").Exec(); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	var runID uuid.UUID
	err := models.DB.Transaction(func(tx *pop.Connection) error {
		run, err := StartResync(tx, "rsync-commit-test", 1)
		if err != nil {
			return err
		}
		runID = run.ID
		// The row must be visible on a DIFFERENT connection already — the
		// outer transaction has not committed yet.
		found := &models.ResyncRun{}
		if err := models.DB.Find(found, run.ID); err != nil {
			t.Errorf("run row not visible outside the request tx before commit: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StartResync: %v", err)
	}

	// The worker goroutine was launched: wait for it to finish so the run
	// does not linger in 'running' (it cancels quickly — the pusher target
	// is unreachable and there is nothing to do, but the row must leave the
	// 'running' state one way or another).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		run := &models.ResyncRun{}
		if err := models.DB.Find(run, runID); err != nil {
			t.Fatalf("run row vanished: %v", err)
		}
		if run.Status != "running" {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Errorf("worker never finished: run still 'running' after 30s")
}
