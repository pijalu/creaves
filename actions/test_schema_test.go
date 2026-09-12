//go:build sqlite
// +build sqlite

package actions

import "strings"

// createReferenceTables adds the application reference/data tables to the
// SQLite test database so the payload-builder, resync, report and sync-status
// fixtures can seed full animal chains (animals, discoveries, intakes,
// outtakes, species, translations, ...). createPusherTables() in
// webhook_pusher_test.go only covers the event-stream subset.
//
// Column definitions mirror migrations/schema.sql; MySQL-specific bits
// (AUTO_INCREMENT, KEY/INDEX clauses, ENGINE) are dropped — but PRIMARY KEY
// constraints on id columns are kept: without them SQLite accepts duplicate
// reference rows, and the resync chunk loader's LEFT JOINs would multiply
// animal rows (the production MySQL schema has these PKs).
// WARNING: keep in sync with migrations/schema.sql when columns are added.
func createReferenceTables() {
	must := func(q string) {
		if err := pusherTestDB.RawQuery(q).Exec(); err != nil {
			panic("test schema: " + err.Error() + "\n" + q)
		}
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS animals (

  "id" integer PRIMARY KEY,
  "species" varchar(255) NOT NULL,
  "ring" varchar(255) DEFAULT NULL,
  "cage" varchar(255) DEFAULT NULL,
  "animalage_id" char(36) NOT NULL,
  "animaltype_id" char(36) NOT NULL,
  "discovery_id" char(36) NOT NULL,
  "intake_id" char(36) NOT NULL,
  "outtake_id" char(36) DEFAULT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "feeding" varchar(255) DEFAULT NULL,
  "gender" varchar(255) DEFAULT NULL,
  "year" int DEFAULT NULL,
  "yearNumber" int DEFAULT NULL,
  "IntakeDate" datetime NOT NULL,
  "force_feed" tinyint(1) NOT NULL DEFAULT '0',
  "zone" varchar(255) DEFAULT NULL,
  "feeding_start" datetime DEFAULT NULL,
  "feeding_end" datetime DEFAULT NULL,
  "feeding_period" int NOT NULL DEFAULT '0'
)`,
		`CREATE TABLE IF NOT EXISTS animalages (

  "id" char(36) PRIMARY KEY,
  "name" varchar(255) NOT NULL,
  "description" text,
  "def" tinyint(1) NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS animaltypes (

  "id" char(36) PRIMARY KEY,
  "name" varchar(255) NOT NULL,
  "description" text,
  "def" tinyint(1) NOT NULL DEFAULT '0',
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "has_ring" tinyint(1) NOT NULL DEFAULT '0',
  "default_species" varchar(255) DEFAULT NULL
)`,
		`CREATE TABLE IF NOT EXISTS species (

  "ID" varchar(255) NOT NULL,
  "species" varchar(255) NOT NULL,
  "class" varchar(255) NOT NULL,
  "family" varchar(255) NOT NULL,
  "creaves_species" varchar(255) NOT NULL,
  "subside_group" varchar(255) NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "order" varchar(255) NOT NULL,
  "game" tinyint(1) NOT NULL DEFAULT '0',
  "agw_group" varchar(255) NOT NULL,
  "native_status" varchar(255) NOT NULL,
  "huntable" tinyint(1) NOT NULL DEFAULT '0',
  "animaltype_id" char(36) DEFAULT NULL
  )`,
		`CREATE TABLE IF NOT EXISTS translations (

  "id" varchar(36) NOT NULL,
  "table_name" varchar(64) NOT NULL,
  "record_id" varchar(36) NOT NULL,
  "field" varchar(64) NOT NULL,
  "locale" varchar(8) NOT NULL,
  "value" text NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS discoveries (

  "id" char(36) PRIMARY KEY,
  "location" varchar(255) DEFAULT NULL,
  "date" datetime NOT NULL,
  "reason" text,
  "note" text,
  "discoverer_id" char(36) NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "postal_code" varchar(255) DEFAULT NULL,
  "city" varchar(255) DEFAULT NULL,
  "return_habitat" tinyint(1) NOT NULL DEFAULT '0',
  "in_garden" tinyint(1) NOT NULL DEFAULT '0',
  "entry_cause_id" varchar(255) NOT NULL DEFAULT '1.1'
)`,
		`CREATE TABLE IF NOT EXISTS discoverers (

  "id" char(36) PRIMARY KEY,
  "firstname" varchar(255) DEFAULT NULL,
  "lastname" varchar(255) DEFAULT NULL,
  "address" varchar(255) DEFAULT NULL,
  "city" varchar(255) DEFAULT NULL,
  "country" varchar(255) DEFAULT NULL,
  "email" varchar(255) DEFAULT NULL,
  "phone" varchar(255) DEFAULT NULL,
  "note" text,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "postal_code" varchar(255) DEFAULT NULL,
  "return_request" tinyint(1) NOT NULL DEFAULT '0',
  "donation" varchar(255) DEFAULT NULL
)`,
		`CREATE TABLE IF NOT EXISTS entry_causes (

  "id" varchar(255) PRIMARY KEY,
  "cause" varchar(255) NOT NULL,
  "detail" varchar(255) NOT NULL,
  "nature" varchar(255) NOT NULL,
  "indication" varchar(255) NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "sort_order" int NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS intakes (

  "id" char(36) PRIMARY KEY,
  "date" datetime NOT NULL,
  "general" text,
  "wounds" text,
  "parasites" text,
  "remarks" text,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "has_wounds" tinyint(1) NOT NULL DEFAULT '0',
  "has_parasites" tinyint(1) NOT NULL DEFAULT '0'
)`,
		`CREATE TABLE IF NOT EXISTS outtakes (

  "id" char(36) PRIMARY KEY,
  "date" datetime NOT NULL,
  "outtaketype_id" char(36) NOT NULL,
  "location" varchar(255) DEFAULT NULL,
  "note" text,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS outtaketypes (

  "id" char(36) PRIMARY KEY,
  "name" varchar(255) NOT NULL,
  "code" varchar(255) DEFAULT NULL,
  "description" text,
  "def" tinyint(1) NOT NULL DEFAULT '0',
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL,
  "dead" tinyint(1) NOT NULL DEFAULT '0',
  "rating" int NOT NULL DEFAULT '0',
  "discoverer_news" text,
  "error" tinyint(1) NOT NULL DEFAULT '0'
)`,

		// Sub-tables used by the annual-report fixtures (taxonomy groups).
		`CREATE TABLE IF NOT EXISTS subside_groups (

  "id" varchar(255) NOT NULL,
  "group" varchar(255) NOT NULL,
  "size" int NOT NULL,
  "amount" float NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`,

		`CREATE TABLE IF NOT EXISTS native_statuses (

  "id" varchar(255) NOT NULL,
  "status" varchar(255) NOT NULL,
  "indication" varchar(255) NOT NULL,
  "precision" varchar(255) DEFAULT NULL,
  "freeable" tinyint(1) NOT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`,
	} {
		must(q)
	}
	if err := pusherTestDB.RawQuery(`ALTER TABLE outtaketypes ADD COLUMN code varchar(255)`).Exec(); err != nil {
		// Existing test DBs may already include the column; fixture setup remains idempotent.
		if !strings.Contains(err.Error(), "duplicate column") {
			panic("test schema: " + err.Error())
		}
	}
}
