package actions

import (
	"strings"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

type payloadTranslationField struct {
	name, table, dbField, id, base string
}

// payloadTranslationFieldsFor lists every payload field that can carry a
// stored translation. Species-derived fields are keyed by the species row id
// (species.ID, e.g. "SP1") — the translations table stores species rows under
// their business key, not under the French display name kept on
// animals.species. species may be nil (unknown species → lookups skipped,
// fr base fallback still fills the canonical values).
func payloadTranslationFieldsFor(animal *models.Animal, payload *models.EventPayload, species *models.Species) []payloadTranslationField {
	speciesID := ""
	if species != nil {
		speciesID = species.ID
	}
	return []payloadTranslationField{
		{name: "species", table: "species", dbField: "creaves_species", id: speciesID, base: animal.Species},
		{name: "animal_type", table: "animaltypes", dbField: "name", id: animal.Animaltype.ID.String(), base: payload.Animal.AnimalType},
		{name: "animal_age", table: "animalages", dbField: "name", id: animal.Animalage.ID.String(), base: payload.Animal.AnimalAge},
		{name: "zone", table: "zones", dbField: "zone", id: payload.Animal.Zone, base: payload.Animal.Zone},
		{name: "outtake_type", table: "outtaketypes", dbField: "name", id: payload.Outtake.TypeID, base: payload.Outtake.Type},
		{name: "entry_cause", table: "entry_causes", dbField: "cause", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCause},
		{name: "entry_cause_detail", table: "entry_causes", dbField: "detail", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCauseDetail},
		{name: "entry_cause_nature", table: "entry_causes", dbField: "nature", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCauseNature},
		{name: "species_class", table: "species", dbField: "class", id: speciesID, base: payload.Animal.SpeciesClass},
		{name: "species_agw_group", table: "species", dbField: "agw_group", id: speciesID, base: payload.Animal.SpeciesAGWGroup},
		{name: "species_subside_group", table: "species", dbField: "subside_group", id: speciesID, base: payload.Animal.SpeciesSubsideGroup},
		{name: "species_native_status", table: "species", dbField: "native_status", id: speciesID, base: payload.Animal.SpeciesNativeStatus},
	}
}

func loadPayloadTranslations(tx *pop.Connection, animal *models.Animal, payload *models.EventPayload, species *models.Species) map[string]map[string]string {
	fields := payloadTranslationFieldsFor(animal, payload, species)
	result := map[string]map[string]string{}
	any := false
	for _, locale := range models.SupportedLocales {
		values := map[string]string{}
		for _, field := range fields {
			if field.id == "" || field.id == uuid.Nil.String() {
				continue
			}
			loaded, err := models.LoadTranslations(tx, field.table, field.dbField, locale, []string{field.id})
			if err == nil && loaded[field.id] != "" {
				values[field.name] = loaded[field.id]
				any = true
			}
		}
		if locale == "fr" {
			for _, field := range fields {
				if field.base != "" {
					values[field.name] = field.base
				}
			}
		}
		result[locale] = values
	}
	if !any {
		return nil
	}
	return result
}

// translationPreloader batches the per-reference-table translation lookups and
// the species taxonomy lookup for a whole set of animals (one resync run, one
// sync-status computation) into a fixed number of queries: one
// models.LoadTranslations call per (table, field) group per locale, plus one
// species table scan. Without it, buildEventPayloadWithTranslations issued
// up to 48 translation SELECTs + 1 species SELECT per animal — a full resync
// of 10k animals flooded the SQL log with ~500k per-record queries.
type translationPreloader struct {
	// species maps animals.species (creaves_species) -> reference row.
	species map[string]*models.Species
	// values maps locale -> "table|field" -> record_id -> translated value.
	values map[string]map[string]map[string]string
}

// newTranslationPreloader loads every translation and species row the given
// animals can reference. tx must be non-nil. Query failures on individual
// groups are ignored (missing translations fall back to base values) — the
// same leniency as the per-animal path.
func newTranslationPreloader(tx *pop.Connection, animals *models.Animals) *translationPreloader {
	p := &translationPreloader{
		species: map[string]*models.Species{},
		values:  map[string]map[string]map[string]string{},
	}

	// Species reference rows: one unfiltered query for the whole run (the
	// reference table is small). RawQuery is deliberate: pop's generated
	// SELECT enumerates the `order` column unquoted, which the SQLite
	// dialect rejects; SELECT * avoids identifier quoting entirely.
	speciesNames := map[string]bool{}
	for i := range *animals {
		if name := (*animals)[i].Species; name != "" {
			speciesNames[name] = true
		}
	}
	if len(speciesNames) > 0 {
		species := []models.Species{}
		if err := tx.RawQuery("SELECT ID AS id, species, creaves_species, class, `order`, family, native_status, agw_group, subside_group, game, huntable, created_at, updated_at FROM species").All(&species); err == nil {
			for i := range species {
				s := &species[i]
				if speciesNames[s.CreavesSpecies] {
					p.species[s.CreavesSpecies] = s
				}
			}
		}
	}

	// Distinct ids per (table, field) group across all animals. Species
	// translations are keyed by the species row id, resolved above.
	groups := map[string]map[string]bool{}
	add := func(table, field, id string) {
		if id == "" || id == uuid.Nil.String() {
			return
		}
		key := table + "|" + field
		if groups[key] == nil {
			groups[key] = map[string]bool{}
		}
		groups[key][id] = true
	}
	for i := range *animals {
		a := &(*animals)[i]
		if s := p.species[a.Species]; s != nil {
			add("species", "creaves_species", s.ID)
			add("species", "class", s.ID)
			add("species", "agw_group", s.ID)
			add("species", "subside_group", s.ID)
			add("species", "native_status", s.ID)
		}
		add("animaltypes", "name", a.Animaltype.ID.String())
		add("animalages", "name", a.Animalage.ID.String())
		if a.Zone.Valid {
			add("zones", "zone", a.Zone.String)
		}
		if a.Outtake != nil && a.Outtake.Type.ID != uuid.Nil {
			add("outtaketypes", "name", a.Outtake.Type.ID.String())
		}
		if a.Discovery.EntryCauseID != "" {
			add("entry_causes", "cause", a.Discovery.EntryCauseID)
			add("entry_causes", "detail", a.Discovery.EntryCauseID)
			add("entry_causes", "nature", a.Discovery.EntryCauseID)
		}
	}

	// Translations: one query per (group, locale) for the whole run.
	for _, locale := range models.SupportedLocales {
		byGroup := map[string]map[string]string{}
		for key, ids := range groups {
			parts := strings.SplitN(key, "|", 2)
			idList := make([]string, 0, len(ids))
			for id := range ids {
				idList = append(idList, id)
			}
			loaded, err := models.LoadTranslations(tx, parts[0], parts[1], locale, idList)
			if err == nil {
				byGroup[key] = loaded
			}
		}
		p.values[locale] = byGroup
	}
	return p
}

// speciesFor returns the species reference row for a creaves_species name, or
// nil when unknown (payload taxonomy fields stay empty — same as the
// per-animal path's failed First()).
func (p *translationPreloader) speciesFor(creavesName string) *models.Species {
	return p.species[creavesName]
}

// translation returns the translated value for one field of one record, or ""
// when absent.
func (p *translationPreloader) translation(locale, table, field, id string) string {
	byGroup := p.values[locale]
	if byGroup == nil {
		return ""
	}
	loaded := byGroup[table+"|"+field]
	return loaded[id]
}

// loadPayloadTranslationsPreloaded is the batched equivalent of
// loadPayloadTranslations: identical output, zero per-animal queries.
func loadPayloadTranslationsPreloaded(p *translationPreloader, animal *models.Animal, payload *models.EventPayload) map[string]map[string]string {
	fields := payloadTranslationFieldsFor(animal, payload, p.speciesFor(animal.Species))
	result := map[string]map[string]string{}
	any := false
	for _, locale := range models.SupportedLocales {
		values := map[string]string{}
		for _, field := range fields {
			if field.id == "" || field.id == uuid.Nil.String() {
				continue
			}
			if v := p.translation(locale, field.table, field.dbField, field.id); v != "" {
				values[field.name] = v
				any = true
			}
		}
		if locale == "fr" {
			for _, field := range fields {
				if field.base != "" {
					values[field.name] = field.base
				}
			}
		}
		result[locale] = values
	}
	if !any {
		return nil
	}
	return result
}
