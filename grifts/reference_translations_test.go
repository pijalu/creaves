package grifts

import (
	"encoding/csv"
	"strings"
	"testing"

	"creaves/models"
)

// TestReferenceTranslationsEntryCauseCoverage proves every non-empty
// canonical field of every create_entry_cause.csv row has en-US/de/nl
// translations in referenceTranslations. DB-less: parses the embedded CSV.
func TestReferenceTranslationsEntryCauseCoverage(t *testing.T) {
	f, err := embedData.Open("create_entry_cause.csv")
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected header + data rows, got %d", len(rows))
	}
	header := rows[0]
	idx := map[string]int{}
	for i, name := range header {
		idx[strings.TrimSpace(name)] = i
	}

	dataRows := 0
	for _, record := range rows[1:] {
		id := strings.TrimSpace(record[idx["ID"]])
		if id == "" {
			continue
		}
		dataRows++
		recs := referenceTranslations["entry_causes"]
		if recs == nil {
			t.Fatalf("no entry_causes translations at all")
		}
		tr, ok := recs[id]
		if !ok {
			t.Errorf("entry_causes %s: missing translation record", id)
			continue
		}
		for _, field := range []string{"cause", "detail", "nature", "indication"} {
			canonical := strings.TrimSpace(record[idx[field]])
			if canonical == "" {
				continue // empty canonical fields legitimately have no translations
			}
			values, ok := tr[field]
			if !ok {
				t.Errorf("entry_causes %s field %s: no translations for canonical %q", id, field, canonical)
				continue
			}
			for i, locale := range refLocales {
				if strings.TrimSpace(values[i]) == "" {
					t.Errorf("entry_causes %s field %s locale %s: empty value", id, field, locale)
				}
			}
		}
	}
	if dataRows == 0 {
		t.Fatalf("no data rows parsed from CSV")
	}
	if dataRows != len(referenceTranslations["entry_causes"]) {
		t.Errorf("CSV has %d rows but translations map has %d entry_causes records", dataRows, len(referenceTranslations["entry_causes"]))
	}
}

// TestReferenceTranslationsFixedRecords covers native_statuses, subside_groups
// and zones: every non-empty canonical field must have all 3 locales.
func TestReferenceTranslationsFixedRecords(t *testing.T) {
	expected := map[string]map[string][]string{
		// table -> record -> fields (empty canonical fields omitted)
		"native_statuses": {
			"NS1": {"status", "indication"},
			"NS2": {"status", "indication", "precision"},
			"NS3": {"status", "indication", "precision"},
			"NS4": {"status", "indication", "precision"},
			"NS5": {"status", "indication", "precision"},
		},
		"subside_groups": {
			"SG1": {"group"},
			"SG2": {"group"},
			"SG3": {"group"},
			"SG4": {"group"},
		},
	}
	for table, records := range expected {
		for recordID, fields := range records {
			tr, ok := referenceTranslations[table][recordID]
			if !ok {
				t.Fatalf("%s %s: missing translation record", table, recordID)
			}
			for _, field := range fields {
				values, ok := tr[field]
				if !ok {
					t.Errorf("%s %s field %s: no translations", table, recordID, field)
					continue
				}
				for i, locale := range refLocales {
					if strings.TrimSpace(values[i]) == "" {
						t.Errorf("%s %s field %s locale %s: empty value", table, recordID, field, locale)
					}
				}
			}
		}
	}

	// Zones keyed by canonical zone name (create_zones.go seeds these 4).
	for _, zone := range []string{"Centre", "Cabinet vétérinaire", "Soft-release", "Famille d'accueil"} {
		tr, ok := zoneTranslations[zone]
		if !ok {
			t.Errorf("zone %q: no translations", zone)
			continue
		}
		values, ok := tr["zone"]
		if !ok {
			t.Errorf("zone %q: no 'zone' field translations", zone)
			continue
		}
		for i, locale := range refLocales {
			if strings.TrimSpace(values[i]) == "" {
				t.Errorf("zone %q locale %s: empty value", zone, locale)
			}
		}
	}
}

// TestReferenceTranslationsLocales guards the structure: only non-base
// supported locales, fixed order, no stray keys.
func TestReferenceTranslationsLocales(t *testing.T) {
	supported := map[string]bool{}
	for _, l := range models.SupportedLocales {
		supported[l] = true
	}
	for _, locale := range refLocales {
		if !supported[locale] {
			t.Errorf("refLocale %q not in models.SupportedLocales", locale)
		}
		if locale == "fr" {
			t.Errorf("fr is the canonical base locale and must not be in refLocales")
		}
	}
	for table, records := range referenceTranslations {
		for recordID, fields := range records {
			for field, values := range fields {
				if len(values) != len(refLocales) {
					t.Errorf("%s/%s/%s: expected %d locale values, got %d", table, recordID, field, len(refLocales), len(values))
				}
			}
		}
	}
}
