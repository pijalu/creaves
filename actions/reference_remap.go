package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// referenceRemap atomically moves foreign-key references and removes source.
func referenceRemap(c buffalo.Context, table, param string, dependents map[string]string) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	sourceID, err := uuid.FromString(c.Param(param))
	if err != nil {
		return c.Error(http.StatusBadRequest, err)
	}
	input := struct {
		ReplacementID string `json:"replacement_id" form:"replacement_id"`
	}{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	replacementID, err := uuid.FromString(input.ReplacementID)
	if err != nil || replacementID == sourceID {
		return c.Error(http.StatusBadRequest, fmt.Errorf("valid different replacement_id required"))
	}
	var count int
	if err := tx.RawQuery("SELECT COUNT(*) FROM `"+table+"` WHERE id = ?", sourceID).First(&count); err != nil {
		return err
	}
	if count == 0 {
		return c.Error(http.StatusNotFound, fmt.Errorf("source not found"))
	}
	if err := tx.RawQuery("SELECT COUNT(*) FROM `"+table+"` WHERE id = ?", replacementID).First(&count); err != nil {
		return err
	}
	if count == 0 {
		return c.Error(http.StatusNotFound, fmt.Errorf("replacement not found"))
	}
	for dep, column := range dependents {
		if err := tx.RawQuery("UPDATE `"+dep+"` SET `"+column+"` = ? WHERE `"+column+"` = ?", replacementID, sourceID).Exec(); err != nil {
			return err
		}
	}
	if err := tx.RawQuery("DELETE FROM `"+table+"` WHERE id = ?", sourceID).Exec(); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.JSON(map[string]string{"remapped_to": replacementID.String()}))
}

func (v AnimalagesResource) Remap(c buffalo.Context) error {
	err := referenceRemap(c, "animalages", "animalage_id", map[string]string{"animals": "animalage_id"})
	if err == nil {
		InvalidateAnimalagesRefCache()
	}
	return err
}
func (v CaretypesResource) Remap(c buffalo.Context) error {
	err := referenceRemap(c, "caretypes", "caretype_id", map[string]string{"cares": "type_id"})
	if err == nil {
		InvalidateCaretypesRefCache()
	}
	return err
}
func (v OuttaketypesResource) Remap(c buffalo.Context) error {
	err := referenceRemap(c, "outtaketypes", "outtaketype_id", map[string]string{"outtakes": "outtaketype_id"})
	if err == nil {
		InvalidateOuttaketypesRefCache()
	}
	return err
}
func (v TraveltypesResource) Remap(c buffalo.Context) error {
	err := referenceRemap(c, "traveltypes", "traveltype_id", map[string]string{"travels": "traveltype_id"})
	if err == nil {
		InvalidateTraveltypesRefCache()
	}
	return err
}
func (v DiscoverersResource) Remap(c buffalo.Context) error {
	return referenceRemap(c, "discoverers", "discoverer_id", map[string]string{"discoveries": "discoverer_id"})
}
func (v DrugsResource) Remap(c buffalo.Context) error {
	return referenceRemap(c, "drugs", "drug_id", map[string]string{"dosages": "drug_id"})
}
