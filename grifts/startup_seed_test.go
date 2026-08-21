package grifts

import "testing"

func TestStartupTranslationInventory(t *testing.T) {
	want := map[string]int{
		"animalages": 2, "animaltypes": 3, "caretypes": 2,
		"outtaketypes": 3, "drugs": 2, "dosages": 2, "species": 8,
	}
	for table, count := range want {
		if got := len(startupTranslatableFields[table]); got != count {
			t.Fatalf("%s inventory fields = %d, want %d", table, got, count)
		}
	}
}

func TestTranslationFileLocaleRegex(t *testing.T) {
	for _, name := range []string{"translations_en-US.sql", "translations_de.sql", "translations_nl.sql"} {
		if !translationFileLocaleRe.MatchString(name) {
			t.Errorf("locale filename not accepted: %s", name)
		}
	}
	if translationFileLocaleRe.MatchString("translations_en_us.sql") {
		t.Error("underscore locale filename unexpectedly accepted")
	}
}
