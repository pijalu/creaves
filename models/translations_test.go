package models

import (
	"testing"

	"github.com/gofrs/uuid"
)

func TestTranslationValidate(t *testing.T) {
	tr := &Translation{}
	errs, err := tr.Validate(DB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errs.HasAny() {
		t.Fatal("expected validation errors on empty translation")
	}

	tr = &Translation{TableName: "animaltypes", RecordID: "x", Field: "name", Locale: "xx", Value: "v"}
	errs, err = tr.Validate(DB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errs.HasAny() {
		t.Fatal("expected locale validation error for unsupported locale")
	}

	tr = &Translation{TableName: "animaltypes", RecordID: "x", Field: "name", Locale: "de", Value: "Vogel"}
	errs, err = tr.Validate(DB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errs.HasAny() {
		t.Fatalf("unexpected validation errors: %v", errs.Errors)
	}
}

func TestTranslationSaveLoadResolve(t *testing.T) {
	id := uuid.Must(uuid.NewV4()).String()

	// Cleanup first so re-runs are deterministic.
	DB.RawQuery("DELETE FROM translations WHERE table_name = ? AND record_id IN (?, ?)", "animaltypes", id, "t2").Exec()

	if err := SaveTranslation(DB, "animaltypes", id, "name", "de", "Vogel"); err != nil {
		t.Fatalf("save: %v", err)
	}

	tr, err := LoadTranslations(DB, "animaltypes", "name", "de", []string{id, "t2"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tr[id] != "Vogel" {
		t.Fatalf("expected 'Vogel', got %q", tr[id])
	}

	// Resolve: translated value wins for de; base for fr/unknown id.
	if got := ResolveName("de", "Oiseau", tr, id); got != "Vogel" {
		t.Fatalf("resolve de = %q, want Vogel", got)
	}
	if got := ResolveName("fr", "Oiseau", tr, id); got != "Oiseau" {
		t.Fatalf("resolve fr = %q, want Oiseau", got)
	}
	if got := ResolveName("de", "Oiseau", tr, "t2"); got != "Oiseau" {
		t.Fatalf("resolve fallback = %q, want Oiseau", got)
	}

	// Upsert: same key updates value, no duplicate row.
	if err := SaveTranslation(DB, "animaltypes", id, "name", "de", "Adler"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	tr, err = LoadTranslations(DB, "animaltypes", "name", "de", []string{id})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if tr[id] != "Adler" {
		t.Fatalf("expected 'Adler' after upsert, got %q", tr[id])
	}
	cnt, err := DB.Q().Where("table_name = ? AND record_id = ? AND field = ? AND locale = ?", "animaltypes", id, "name", "de").Count(&Translation{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 row for unique key, got %d", cnt)
	}

	// Cleanup.
	DB.RawQuery("DELETE FROM translations WHERE table_name = ? AND record_id = ?", "animaltypes", id).Exec()
}
