package actions

import (
	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

type payloadTranslationField struct {
	name, table, dbField, id, base string
}

func loadPayloadTranslations(tx *pop.Connection, animal *models.Animal, payload *models.EventPayload) map[string]map[string]string {
	fields := []payloadTranslationField{
		{name: "species", table: "species", dbField: "creaves_species", id: animal.Species, base: animal.Species},
		{name: "animal_type", table: "animaltypes", dbField: "name", id: animal.Animaltype.ID.String(), base: payload.Animal.AnimalType},
		{name: "animal_age", table: "animalages", dbField: "name", id: animal.Animalage.ID.String(), base: payload.Animal.AnimalAge},
		{name: "zone", table: "zones", dbField: "zone", id: payload.Animal.Zone, base: payload.Animal.Zone},
		{name: "outtake_type", table: "outtaketypes", dbField: "name", id: payload.Outtake.TypeID, base: payload.Outtake.Type},
		{name: "entry_cause", table: "entry_causes", dbField: "cause", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCause},
		{name: "entry_cause_detail", table: "entry_causes", dbField: "detail", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCauseDetail},
		{name: "entry_cause_nature", table: "entry_causes", dbField: "nature", id: payload.Discovery.EntryCauseID, base: payload.Discovery.EntryCauseNature},
		{name: "species_class", table: "species", dbField: "class", id: animal.Species, base: payload.Animal.SpeciesClass},
		{name: "species_agw_group", table: "species", dbField: "agw_group", id: animal.Species, base: payload.Animal.SpeciesAGWGroup},
		{name: "species_subside_group", table: "species", dbField: "subside_group", id: animal.Species, base: payload.Animal.SpeciesSubsideGroup},
		{name: "species_native_status", table: "species", dbField: "native_status", id: animal.Species, base: payload.Animal.SpeciesNativeStatus},
	}
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
