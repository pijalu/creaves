package actions

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

type referenceDeleteSpec struct {
	Table, IDParam, Label string
	Dependents            map[string]string
}

type referenceDeleteOption struct {
	ID   uuid.UUID
	Name string
}

var referenceDeleteSpecs = []referenceDeleteSpec{
	{"animalages", "animalage_id", "Animal age", map[string]string{"animals": "animalage_id"}},
	{"animaltypes", "animaltype_id", "Animal type", map[string]string{"animals": "animaltype_id", "dosages": "animaltype_id", "species": "animaltype_id"}},
	{"caretypes", "caretype_id", "Care type", map[string]string{"cares": "type_id"}},
	{"discoverers", "discoverer_id", "Discoverer", map[string]string{"discoveries": "discoverer_id"}},
	{"drugs", "drug_id", "Drug", map[string]string{"dosages": "drug_id"}},
	{"outtaketypes", "outtaketype_id", "Outtake type", map[string]string{"outtakes": "outtaketype_id"}},
	{"traveltypes", "traveltype_id", "Travel type", map[string]string{"travels": "traveltype_id"}},
}

func referenceDeleteSpecFor(c buffalo.Context) (referenceDeleteSpec, error) {
	path := strings.Trim(c.Request().URL.Path, "/")
	for _, spec := range referenceDeleteSpecs {
		if strings.HasPrefix(path, spec.Table+"/") {
			return spec, nil
		}
	}
	return referenceDeleteSpec{}, fmt.Errorf("unsupported reference delete path")
}

func referenceDeleteRecord(c buffalo.Context, tx *pop.Connection, spec referenceDeleteSpec) (uuid.UUID, string, error) {
	id, err := uuid.FromString(c.Param(spec.IDParam))
	if err != nil {
		return uuid.Nil, "", c.Error(http.StatusBadRequest, err)
	}
	var row struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	if err := tx.RawQuery("SELECT id, name FROM `"+spec.Table+"` WHERE id = ?", id).First(&row); err != nil {
		return uuid.Nil, "", c.Error(http.StatusNotFound, err)
	}
	return id, row.Name, nil
}

func ReferenceDeleteNew(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	spec, err := referenceDeleteSpecFor(c)
	if err != nil {
		return err
	}
	id, name, err := referenceDeleteRecord(c, tx, spec)
	if err != nil {
		return err
	}
	options := []referenceDeleteOption{}
	if err := tx.RawQuery("SELECT id, name FROM `"+spec.Table+"` WHERE id <> ? ORDER BY name", id).All(&options); err != nil {
		return err
	}
	c.Set("reference_delete_spec", spec)
	c.Set("reference_delete_id", id)
	c.Set("reference_delete_name", name)
	c.Set("reference_delete_options", options)
	return c.Render(http.StatusOK, r.HTML("/references/delete.plush.html"))
}

func ReferenceDeleteCreate(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	spec, err := referenceDeleteSpecFor(c)
	if err != nil {
		return err
	}
	sourceID, _, err := referenceDeleteRecord(c, tx, spec)
	if err != nil {
		return err
	}
	var input struct {
		ReplacementID string `form:"replacement_id" json:"replacement_id"`
	}
	if err := c.Bind(&input); err != nil {
		return err
	}
	var count int
	for table, column := range spec.Dependents {
		if err := tx.RawQuery("SELECT COUNT(*) FROM `"+table+"` WHERE `"+column+"` = ?", sourceID).First(&count); err != nil {
			return err
		}
		if count > 0 && input.ReplacementID == "" {
			return c.Error(http.StatusUnprocessableEntity, fmt.Errorf("replacement required for dependent records"))
		}
	}
	if input.ReplacementID != "" {
		replacementID, err := uuid.FromString(input.ReplacementID)
		if err != nil || replacementID == sourceID {
			return c.Error(http.StatusBadRequest, fmt.Errorf("valid different replacement_id required"))
		}
		if err := tx.RawQuery("SELECT COUNT(*) FROM `"+spec.Table+"` WHERE id = ?", replacementID).First(&count); err != nil || count == 0 {
			return c.Error(http.StatusBadRequest, fmt.Errorf("replacement not found"))
		}
		for table, column := range spec.Dependents {
			if err := tx.RawQuery("UPDATE `"+table+"` SET `"+column+"` = ? WHERE `"+column+"` = ?", replacementID, sourceID).Exec(); err != nil {
				return err
			}
		}
	}
	if err := tx.RawQuery("DELETE FROM `"+spec.Table+"` WHERE id = ?", sourceID).Exec(); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/"+spec.Table)
}
