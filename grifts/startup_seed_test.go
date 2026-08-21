package grifts

import (
	"bufio"
	"strings"
	"testing"
)

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

func TestTranslationArtifactsCoverAuditedWorkload(t *testing.T) {
	const want = 4686
	for _, name := range []string{"translations_en-US.sql", "translations_de.sql", "translations_nl.sql"} {
		data, err := translationSQLFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		count := 0
		s := bufio.NewScanner(strings.NewReader(string(data)))
		for s.Scan() {
			if strings.HasPrefix(s.Text(), "INSERT INTO translations") {
				count++
			}
		}
		if err := s.Err(); err != nil {
			t.Fatalf("scan %s: %v", name, err)
		}
		if count != want {
			t.Errorf("%s rows = %d, want %d", name, count, want)
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
