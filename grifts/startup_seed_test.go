package grifts

import (
	"bytes"
	"compress/gzip"
	"creaves/utils"
	"strings"
	"testing"
)

func TestStartupTranslationInventory(t *testing.T) {
	for table, fields := range startupTranslatableFields {
		seen := map[string]bool{}
		for _, col := range startupTableColumns[table] {
			seen[col] = true
		}
		for _, field := range fields {
			if !seen[field] {
				t.Errorf("%s.%s missing from column inventory", table, field)
			}
		}
	}
}

func startupTranslationKeys(t *testing.T) map[string]bool {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(startupSQLGz))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	_, statements, err := utils.ExtractInsertStatements(gz)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for table, fields := range startupTranslatableFields {
		for _, statement := range statements[table] {
			rows, err := utils.ParseInsertRows(statement, startupTableColumns[table])
			if err != nil {
				t.Fatalf("parse %s: %v", table, err)
			}
			for _, row := range rows {
				id := row["id"]
				if table == "species" {
					id = row["ID"]
				}
				for _, field := range fields {
					if value := row[field]; value != "" && value != "NULL" {
						keys[table+"\x00"+id+"\x00"+field] = true
					}
				}
			}
		}
	}
	return keys
}

func TestTranslationArtifactsCoverStartupInventory(t *testing.T) {
	want := startupTranslationKeys(t)
	columns := []string{"id", "table_name", "record_id", "field", "locale", "value", "created_at", "updated_at"}
	for _, name := range []string{"translations_en-US.sql", "translations_fr.sql", "translations_de.sql", "translations_nl.sql"} {
		data, err := translationSQLFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, statements, err := utils.ExtractInsertStatements(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		locale := strings.TrimSuffix(strings.TrimPrefix(name, "translations_"), ".sql")
		got := map[string]bool{}
		for _, statement := range statements["translations"] {
			rows, err := utils.ParseInsertRows(statement, columns)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			for _, row := range rows {
				if row["locale"] != locale {
					t.Errorf("%s locale %s, want %s", name, row["locale"], locale)
				}
				if row["value"] == "" || row["value"] == "NULL" {
					t.Errorf("%s empty value", name)
				}
				key := row["table_name"] + "\x00" + row["record_id"] + "\x00" + row["field"]
				if !want[key] {
					t.Errorf("%s unexpected key %s", name, key)
				}
				got[key] = true
			}
		}
		if len(got) != len(want) {
			t.Errorf("%s rows %d, want %d", name, len(got), len(want))
		}
		for key := range want {
			if !got[key] {
				t.Errorf("%s missing key %s", name, key)
			}
		}
	}
}

func TestTranslationArtifactPrimaryKeysAreUniqueAcrossLocales(t *testing.T) {
	seen := map[string]string{}
	for _, name := range []string{"translations_en-US.sql", "translations_fr.sql", "translations_de.sql", "translations_nl.sql"} {
		data, err := translationSQLFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, statements, err := utils.ExtractInsertStatements(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range statements["translations"] {
			rows, err := utils.ParseInsertRows(statement, []string{"id", "table_name", "record_id", "field", "locale", "value", "created_at", "updated_at"})
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if previous, ok := seen[row["id"]]; ok {
					t.Errorf("translation primary key %s reused by %s and %s", row["id"], previous, name)
				}
				seen[row["id"]] = name
			}
		}
	}
}

func TestTranslationFileLocaleRegex(t *testing.T) {
	for _, name := range []string{"translations_en-US.sql", "translations_fr.sql", "translations_de.sql", "translations_nl.sql"} {
		if !translationFileLocaleRe.MatchString(name) {
			t.Errorf("locale filename not accepted: %s", name)
		}
	}
	if translationFileLocaleRe.MatchString("translations_en_us.sql") {
		t.Error("underscore locale filename unexpectedly accepted")
	}
}
