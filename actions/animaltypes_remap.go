package actions

import (
	"fmt"
	"net/http"

	"creaves/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// AnimaltypesRemap moves every known dependent reference to replacement before
// deleting source. All operations share the request transaction.
func AnimaltypesRemap(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	sourceID, err := uuid.FromString(c.Param("animaltype_id"))
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
	source := &models.Animaltype{}
	replacement := &models.Animaltype{}
	if err := tx.Find(source, sourceID); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := tx.Find(replacement, replacementID); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	// Collect the affected animals before the FK updates rewire them
	// (species/dosages carry no animal linkage and are skipped).
	affected, err := referenceAffectedAnimalIDs(tx, map[string]string{"animals": "animaltype_id"}, sourceID)
	if err != nil {
		return err
	}
	for _, q := range []string{
		"UPDATE animals SET animaltype_id = ? WHERE animaltype_id = ?",
		"UPDATE dosages SET animaltype_id = ? WHERE animaltype_id = ?",
		"UPDATE species SET animaltype_id = ? WHERE animaltype_id = ?",
	} {
		if err := tx.RawQuery(q, replacementID, sourceID).Exec(); err != nil {
			return err
		}
	}
	if err := tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", sourceID).Exec(); err != nil {
		return err
	}
	queuePostCommitInvalidation(c, InvalidateAnimaltypesRefCache)
	publishReferenceStateEvents(c, tx, affected)
	return c.Render(http.StatusOK, r.JSON(map[string]string{"remapped_to": replacementID.String()}))
}
