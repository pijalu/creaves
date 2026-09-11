package grifts

import (
	"encoding/csv"
	"fmt"
	"strings"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
)

func normalizedMappingName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

// repairSpeciesAnimaltypeLinks repairs only empty links. It deliberately
// requires approved mapping rows, preventing candidate Phase 1 data from
// silently changing production classifications.
func repairSpeciesAnimaltypeLinks() error {
	file, err := embedData.Open("species_animaltype_mapping.csv")
	if err != nil {
		return err
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("empty species mapping")
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[name] = i
	}
	required := []string{"species_name", "animal_type_name", "confidence", "review_status"}
	for _, name := range required {
		if _, ok := header[name]; !ok {
			return fmt.Errorf("mapping missing column %s", name)
		}
	}
	return models.DB.Transaction(func(tx *pop.Connection) error {
		for _, row := range rows[1:] {
			if len(row) <= header["review_status"] || row[header["confidence"]] != "high" || row[header["review_status"]] != "approved" {
				return fmt.Errorf("mapping row for %q is not approved", row[header["species_name"]])
			}
			var count int
			if err := tx.RawQuery("SELECT COUNT(*) FROM species WHERE LOWER(TRIM(creaves_species)) = ? AND animaltype_id IS NULL", normalizedMappingName(row[header["species_name"]])).First(&count); err != nil {
				return err
			}
			if count == 0 {
				continue
			}
			var species models.Species
			if err := tx.RawQuery("SELECT ID AS id, species, class, `order`, family, creaves_species, subside_group, agw_group, native_status, game, huntable FROM species WHERE LOWER(TRIM(creaves_species)) = ? AND animaltype_id IS NULL LIMIT 1", normalizedMappingName(row[header["species_name"]])).First(&species); err != nil {
				return err
			}
			var at models.Animaltype
			if err := tx.RawQuery("SELECT * FROM animaltypes WHERE LOWER(TRIM(name)) = ? LIMIT 1", normalizedMappingName(row[header["animal_type_name"]])).First(&at); err != nil {
				return fmt.Errorf("species %q: animal type %q: %w", species.CreavesSpecies, row[header["animal_type_name"]], err)
			}
			if err := tx.RawQuery("UPDATE species SET animaltype_id = ? WHERE ID = ? AND animaltype_id IS NULL", at.ID, species.ID).Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}
