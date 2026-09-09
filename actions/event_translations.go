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
//
// The preloader is INCREMENTAL across chunks of one run: ensure() only
// queries what is not loaded yet, so chunk 2..N of a resync re-uses chunk 1's
// reference data (species/localities full scans happen once per run, and
// translation groups are queried only for newly seen record ids). Reference
// data is static for the lifetime of a run, so caching cannot go stale.
type translationPreloader struct {
	// species maps animals.species (creaves_species) -> reference row.
	species map[string]*models.Species
	// localities maps discoveries.city -> localities reference row (same
	// join as the stat_communes export: d.city = locality).
	localities map[string]*models.Locality
	// values maps locale -> "table|field" -> record_id -> translated value.
	values map[string]map[string]map[string]string
	// coverage tracking for the incremental ensure(): species/locality names
	// already covered by a full reference scan, and (table|field -> ids)
	// translation keys already queried per locale.
	speciesScanned    map[string]bool
	citiesScanned     map[string]bool
	translatedLocales map[string]map[string]map[string]bool // locale -> key -> ids queried
}

// newTranslationPreloader returns an EMPTY preloader; call ensure(tx, animals)
// before first use. Chunk loops build it once and ensure() every chunk.
func newTranslationPreloader() *translationPreloader {
	return &translationPreloader{
		species:           map[string]*models.Species{},
		localities:        map[string]*models.Locality{},
		values:            map[string]map[string]map[string]string{},
		speciesScanned:    map[string]bool{},
		citiesScanned:     map[string]bool{},
		translatedLocales: map[string]map[string]map[string]bool{},
	}
}

// newLoadedTranslationPreloader loads every translation and species row the
// given animals can reference. tx must be non-nil. Query failures on
// individual groups are ignored (missing translations fall back to base
// values) — the same leniency as the per-animal path.
func newLoadedTranslationPreloader(tx *pop.Connection, animals *models.Animals) *translationPreloader {
	p := newTranslationPreloader()
	p.ensure(tx, animals)
	return p
}

// ensure loads the reference data the given animals need that is not loaded
// yet. Safe to call repeatedly with successive chunks: everything already
// covered is skipped, so steady-state cost per chunk is zero queries.
func (p *translationPreloader) ensure(tx *pop.Connection, animals *models.Animals) {
	// Species reference rows: one unfiltered query covering all names seen so
	// far (the reference table is small). RawQuery is deliberate: pop's
	// generated SELECT enumerates the `order` column unquoted, which the
	// SQLite dialect rejects; SELECT * avoids identifier quoting entirely.
	// Re-scanned only when a chunk references unknown names.
	speciesNames := map[string]bool{}
	for i := range *animals {
		if name := (*animals)[i].Species; name != "" && !p.speciesScanned[name] {
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
		for name := range speciesNames {
			p.speciesScanned[name] = true // covered by this scan (present or not)
		}
	}

	// Locality reference rows: one unfiltered scan covering all cities seen
	// so far (the table is small), keyed by the `locality` column the
	// stat_communes export joins discoveries.city on.
	cityNames := map[string]bool{}
	for i := range *animals {
		if c := (*animals)[i].Discovery.City; c.Valid && c.String != "" && !p.citiesScanned[c.String] {
			cityNames[c.String] = true
		}
	}
	if len(cityNames) > 0 {
		localities := []models.Locality{}
		if err := tx.RawQuery("SELECT id, country, region, province, municipality, sub_municipality, postal_code, locality, zoning, direction, created_at, updated_at FROM localities").All(&localities); err == nil {
			for i := range localities {
				l := &localities[i]
				if cityNames[l.Locality] {
					p.localities[l.Locality] = l
				}
			}
		}
		for name := range cityNames {
			p.citiesScanned[name] = true // covered by this scan (present or not)
		}
	}

	// Distinct ids per (table, field) group across all animals — filtered
	// down to ids not loaded yet (incremental across chunks). Species
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

	// Translations: one query per (group, locale) — restricted to ids not
	// already loaded by a previous ensure() of the same run. Loaded values
	// are merged into the existing maps; reference data is static during a
	// run so already-fetched translations cannot go stale.
	for _, locale := range models.SupportedLocales {
		if p.values[locale] == nil {
			p.values[locale] = map[string]map[string]string{}
		}
		if p.translatedLocales[locale] == nil {
			p.translatedLocales[locale] = map[string]map[string]bool{}
		}
		for key, ids := range groups {
			parts := strings.SplitN(key, "|", 2)
			seen := p.translatedLocales[locale][key]
			if seen == nil {
				seen = map[string]bool{}
				p.translatedLocales[locale][key] = seen
			}
			idList := make([]string, 0, len(ids))
			for id := range ids {
				if !seen[id] {
					idList = append(idList, id)
				}
			}
			if len(idList) == 0 {
				continue
			}
			loaded, err := models.LoadTranslations(tx, parts[0], parts[1], locale, idList)
			if err != nil {
				continue // same leniency as before: fall back to base values
			}
			for _, id := range idList {
				seen[id] = true // queried: absent result means no translation exists
			}
			if p.values[locale][key] == nil {
				p.values[locale][key] = map[string]string{}
			}
			for id, value := range loaded {
				p.values[locale][key][id] = value
			}
		}
	}
}

// speciesFor returns the species reference row for a creaves_species name, or
// nil when unknown (payload taxonomy fields stay empty — same as the
// per-animal path's failed First()).
func (p *translationPreloader) speciesFor(creavesName string) *models.Species {
	return p.species[creavesName]
}

// localityFor returns the localities reference row matching a discovery city
// (join key of the stat_communes export), or nil when unknown — payload
// locality fields stay empty, mirroring the export's LEFT JOIN.
func (p *translationPreloader) localityFor(city string) *models.Locality {
	return p.localities[city]
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
