package grifts

import (
	"encoding/json"
	"fmt"

	"creaves/models"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

// E2E fixture seed (plan §7.2).
//
// Produces a fixed, fully known dataset so the agent-browser e2e suite can
// assert exact numbers on /animals, /reports/annual and CSV exports.
// All marker values are prefixed E2E_ so they never collide with real data.
//
// Fixture layout (see e2e/EXPECTATIONS.md for the full per-table numbers):
//
//	Reference rows:
//	  animalages:    E2E Juvenile, E2E Adult
//	  animaltypes:   E2E_TA, E2E_TB
//	  outtaketypes:  E2E_REL (rating 1, alive), E2E_DCD (rating -1, dead),
//	                 E2E_ERR (error=1 -> excluded from every report table)
//	  subside_group: E2E_SG ("E2E Group A")
//	  native_status: E2E_NS ("E2E Native")
//	  entry_causes:  E2E_EC1 (N1/C1/D1), E2E_EC2 (N1/C2, no detail)
//	  species:       E2E_Hedgehog (E2E_Mammalia, E2E_G1, E2E_SG, E2E_NS),
//	                 E2E_Sparrow  (E2E_Aves, E2E_G2, no subside, no native),
//	                 E2E_Newt     (E2E_Aves, E2E_G1, E2E_SG, E2E_NS)
//
//	Animals 2025 (ids 250001..250007):
//	  1 E2E_Hedgehog Juvenile EC1 outtake E2E_REL  ring E2E-RING-001,
//	    city `E2E City; "Nord"` (CSV escaping fixture)
//	  2 E2E_Hedgehog Juvenile EC1 outtake E2E_DCD
//	  3 E2E_Sparrow  Adult    EC2 outtake E2E_REL
//	  4 E2E_Sparrow  Adult    EC2 (no outtake)
//	  5 E2E_Newt     Juvenile EC1 outtake E2E_ERR (excluded everywhere)
//	  6 E2E_NOSPEC   Juvenile EC1 (no outtake, species without taxonomy row,
//	    NULL ring / gender / city / discovery location)
//	  7 E2E_Newt     Adult    EC2 (no outtake)
//
//	Animals 2024 (ids 240008..240009):
//	  8 E2E_Hedgehog Juvenile EC1 (no outtake)
//	  9 E2E_Sparrow  Adult    EC2 outtake E2E_REL
//
// In-scope 2025 = animals 1,2,3,4,6,7 => T=6 for non-outtake tables, T=3 for
// outtake tables (REL x2, DCD x1). 2024 => T=2 / T=1.
//
// The task also points the active config at the local console
// (webhook http://127.0.0.1:3001/webhook/events, key E2EConsoleKey) and sets
// instance_id=e2e-instance-a, so a resync feeds the console through the real
// webhook path. Run against a fresh database:
//
//	buffalo pop drop && buffalo pop create && buffalo pop migrate
//	buffalo task db:seed && buffalo task db:seed:e2e

// E2EConsoleKey is the raw webhook key the pusher sends; the console's
// db:seed:e2e stores the matching bcrypt hash.
const E2EConsoleKey = "e2e-console-key-0123456789"

// E2EInstanceID is the instance id this fixture set reports under.
const E2EInstanceID = "e2e-instance-a"

func e2eExec(tx *pop.Connection, q string, args ...interface{}) error {
	if err := tx.RawQuery(q, args...).Exec(); err != nil {
		return fmt.Errorf("e2e seed: %w\nquery: %s", err, q)
	}
	return nil
}

func e2eUUID(i, prefix int) string {
	const digits = "0123456789abcdef"
	p := digits[prefix]
	c := digits[i]
	pp := make([]byte, 8)
	for k := range pp {
		pp[k] = p
	}
	return string(pp) + "-0000-0000-0000-0000000000" + string([]byte{c, c})
}

type e2eAnimal struct {
	idx            int
	id, year       int
	species, age   string
	animaltype     string
	entryCause     string
	outtake        string // "" = none
	ring           string // "" = NULL
	gender         string // "" = NULL
	city           string // "" = NULL
	discoveryPlace string // "" = NULL
}

func e2eFixtures() []e2eAnimal {
	const (
		ageYoung = "aaaaaaaa-0000-0000-0000-0000000000e1"
		ageAdult = "aaaaaaaa-0000-0000-0000-0000000000e2"
		typeA    = "bbbbbbbb-0000-0000-0000-0000000000e1"
		typeB    = "bbbbbbbb-0000-0000-0000-0000000000e2"
		rel      = "cccccccc-0000-0000-0000-0000000000e1"
		dcd      = "cccccccc-0000-0000-0000-0000000000e2"
		errT     = "cccccccc-0000-0000-0000-0000000000e3"
	)
	return []e2eAnimal{
		{0, 250001, 2025, "E2E_Hedgehog", ageYoung, typeA, "E2E_EC1", rel, "E2E-RING-001", "male", `E2E City; "Nord"`, "E2E Hedge; \"Lane\""},
		{1, 250002, 2025, "E2E_Hedgehog", ageYoung, typeA, "E2E_EC1", dcd, "E2E-RING-002", "female", "E2E Town", ""},
		{2, 250003, 2025, "E2E_Sparrow", ageAdult, typeB, "E2E_EC2", rel, "E2E-RING-003", "male", "E2E Town", ""},
		{3, 250004, 2025, "E2E_Sparrow", ageAdult, typeB, "E2E_EC2", "", "E2E-RING-004", "female", "E2E Village", ""},
		{4, 250005, 2025, "E2E_Newt", ageYoung, typeB, "E2E_EC1", errT, "E2E-RING-005", "male", "E2E Town", ""},
		{5, 250006, 2025, "E2E_NOSPEC", ageYoung, typeA, "E2E_EC1", "", "", "", "", ""},
		{6, 250007, 2025, "E2E_Newt", ageAdult, typeB, "E2E_EC2", "", "E2E-RING-007", "female", "E2E Village", ""},
		{7, 240008, 2024, "E2E_Hedgehog", ageYoung, typeA, "E2E_EC1", "", "E2E-RING-008", "male", "E2E Town", ""},
		{8, 240009, 2024, "E2E_Sparrow", ageAdult, typeB, "E2E_EC2", rel, "E2E-RING-009", "female", "E2E Village", ""},
	}
}

var _ = grift.Namespace("db", func() {
	grift.Desc("seed:e2e", "Seeds fixed E2E fixture data (plan §7.2) on top of db:seed")
	grift.Add("seed:e2e", func(c *grift.Context) error {
		return models.DB.Transaction(func(tx *pop.Connection) error {
			exists, err := tx.Where("ID = ?", "E2E_Hedgehog").Exists(&models.Species{})
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("E2E fixtures already present; reset the database first (buffalo pop drop && buffalo pop create && buffalo pop migrate && buffalo task db:seed)")
			}

			now := "NOW()"

			// Reference data
			if err := e2eExec(tx, "INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('aaaaaaaa-0000-0000-0000-0000000000e1', 'E2E Juvenile', 0, "+now+", "+now+"), ('aaaaaaaa-0000-0000-0000-0000000000e2', 'E2E Adult', 0, "+now+", "+now+")"); err != nil {
				return err
			}
			if err := e2eExec(tx, "INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('bbbbbbbb-0000-0000-0000-0000000000e1', 'E2E_TA', 0, "+now+", "+now+"), ('bbbbbbbb-0000-0000-0000-0000000000e2', 'E2E_TB', 0, "+now+", "+now+")"); err != nil {
				return err
			}
			if err := e2eExec(tx, "INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('cccccccc-0000-0000-0000-0000000000e1', 'E2E_REL', 0, "+now+", "+now+", 0, 1, 0), ('cccccccc-0000-0000-0000-0000000000e2', 'E2E_DCD', 0, "+now+", "+now+", 1, -1, 0), ('cccccccc-0000-0000-0000-0000000000e3', 'E2E_ERR', 0, "+now+", "+now+", 0, 0, 1)"); err != nil {
				return err
			}
			if err := e2eExec(tx, "INSERT INTO subside_groups (id, `group`, size, amount, created_at, updated_at) VALUES ('E2E_SG', 'E2E Group A', 1, 50, "+now+", "+now+")"); err != nil {
				return err
			}
			if err := e2eExec(tx, "INSERT INTO native_statuses (id, status, indication, created_at, updated_at, freeable) VALUES ('E2E_NS', 'E2E Native', 'x', "+now+", "+now+", 1)"); err != nil {
				return err
			}
			if err := e2eExec(tx, "INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('E2E_EC1', 'E2E_C1', 'E2E_D1', 'E2E_N1', 'x', "+now+", "+now+", 900), ('E2E_EC2', 'E2E_C2', '', 'E2E_N1', 'x', "+now+", "+now+", 901)"); err != nil {
				return err
			}

			// Species taxonomy rows (E2E_NOSPEC intentionally absent)
			species := []struct{ id, class, subside, agw, native string }{
				{"E2E_Hedgehog", "E2E_Mammalia", "E2E_SG", "E2E_G1", "E2E_NS"},
				{"E2E_Sparrow", "E2E_Aves", "", "E2E_G2", ""},
				{"E2E_Newt", "E2E_Aves", "E2E_SG", "E2E_G1", "E2E_NS"},
			}
			for _, s := range species {
				if err := e2eExec(tx, "INSERT INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, agw_group, native_status) VALUES (?, ?, ?, 'E2E Family', ?, ?, "+now+", "+now+", 'E2E Order', ?, ?)",
					s.id, s.id+" sp.", s.class, s.id, s.subside, s.agw, s.native); err != nil {
					return err
				}
			}

			if err := e2eExec(tx, "INSERT INTO discoverers (id, created_at, updated_at) VALUES ('dddddddd-0000-0000-0000-0000000000e1', "+now+", "+now+")"); err != nil {
				return err
			}

			// Translations for E2E values (en-US, de, nl) so the payload
			// translations map is non-empty and the console can render
			// localized fixture values in the 4-language spot checks.
			// fr is canonical (no translation row needed).
			type e2eTr struct{ table, recordID, field string }
			trTargets := []e2eTr{
				{"species", "E2E_Hedgehog", "creaves_species"},
				{"species", "E2E_Sparrow", "creaves_species"},
				{"species", "E2E_Newt", "creaves_species"},
				{"species", "E2E_Hedgehog", "class"},
				{"animalages", "aaaaaaaa-0000-0000-0000-0000000000e1", "name"},
				{"animalages", "aaaaaaaa-0000-0000-0000-0000000000e2", "name"},
				{"animaltypes", "bbbbbbbb-0000-0000-0000-0000000000e1", "name"},
				{"animaltypes", "bbbbbbbb-0000-0000-0000-0000000000e2", "name"},
				{"outtaketypes", "cccccccc-0000-0000-0000-0000000000e1", "name"},
				{"outtaketypes", "cccccccc-0000-0000-0000-0000000000e2", "name"},
				{"entry_causes", "E2E_EC1", "cause"},
				{"entry_causes", "E2E_EC1", "detail"},
				{"entry_causes", "E2E_EC1", "nature"},
				{"entry_causes", "E2E_EC2", "cause"},
				{"entry_causes", "E2E_EC2", "nature"},
			}
			locales := map[string]string{"en-US": "EN", "de": "DE", "nl": "NL"}
			i := 0
			for _, tg := range trTargets {
				for loc, suffix := range locales {
					i++
					value := fmt.Sprintf("%s|%s|%s|%s", tg.table, tg.recordID, tg.field, suffix)
					if err := e2eExec(tx, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, "+now+", "+now+")",
						fmt.Sprintf("eeeeeeee-0000-0000-0000-%012d", i), tg.table, tg.recordID, tg.field, loc, value); err != nil {
						return err
					}
				}
			}

			for _, a := range e2eFixtures() {
				intakeID := e2eUUID(a.idx, 5)
				discoveryID := e2eUUID(a.idx, 6)
				outtakeID := e2eUUID(a.idx, 7)
				date := fmt.Sprintf("%d-06-15 10:00:00", a.year)

				if err := e2eExec(tx, "INSERT INTO intakes (id, date, created_at, updated_at) VALUES (?, ?, "+now+", "+now+")", intakeID, date); err != nil {
					return err
				}
				var city, place interface{}
				if a.city != "" {
					city = a.city
				}
				if a.discoveryPlace != "" {
					place = a.discoveryPlace
				}
				if err := e2eExec(tx, "INSERT INTO discoveries (id, date, discoverer_id, entry_cause_id, city, location, created_at, updated_at) VALUES (?, ?, 'dddddddd-0000-0000-0000-0000000000e1', ?, ?, ?, "+now+", "+now+")", discoveryID, date, a.entryCause, city, place); err != nil {
					return err
				}
				var outtakeArg interface{}
				if a.outtake != "" {
					if err := e2eExec(tx, "INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES (?, ?, ?, "+now+", "+now+")", outtakeID, date, a.outtake); err != nil {
						return err
					}
					outtakeArg = outtakeID
				}
				var ring, gender interface{}
				if a.ring != "" {
					ring = a.ring
				}
				if a.gender != "" {
					gender = a.gender
				}
				if err := e2eExec(tx, "INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, outtake_id, ring, gender, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, "+now+", "+now+", ?, ?, ?)",
					a.id, a.species, a.age, a.animaltype, discoveryID, intakeID, outtakeArg, ring, gender, a.year, a.id%1000, date); err != nil {
					return err
				}
			}

			// Point the active config at the local console so a resync feeds it
			// through the real webhook path.
			settings := map[string]interface{}{
				"enable_event_stream": true,
				"webhook_enabled":     true,
				"webhook_url":         "http://127.0.0.1:3001/webhook/events",
				"webhook_api_key":     E2EConsoleKey,
				"webhook_batch_size":  10,
				"webhook_max_per_min": 600,
			}
			raw, err := json.Marshal(settings)
			if err != nil {
				return err
			}
			n, err := tx.RawQuery("UPDATE config SET instance_id = ?, settings = ? WHERE active = 1", E2EInstanceID, string(raw)).ExecWithCount()
			if err != nil {
				return err
			}
			if n == 0 {
				return fmt.Errorf("no active config row; run buffalo task db:seed first")
			}

			fmt.Println("E2E fixtures seeded: 9 animals (2025 x7 incl. 1 error-outtake, 2024 x2), instance e2e-instance-a, webhook -> http://127.0.0.1:3001/webhook/events")
			return nil
		})
	})
})
