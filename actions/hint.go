package actions

import (
	"fmt"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

const HINT_SPECIES_DETAILS = "SELECT distinct ns.id, ns.status, ns.indication, ns.`precision`, ns.freeable, s.game, s.huntable " +
	"FROM native_statuses ns " +
	"JOIN species s ON s.native_status=ns.ID " +
	"where s.creaves_species = ?"

// speciesHint is one row of the native-status hint popup. ID is internal —
// used only to look up translations — and not serialized to the client.
type speciesHint struct {
	ID         string       `json:"-" db:"id"`
	Status     string       `json:"status" db:"status"`
	Indication string       `json:"indication" db:"indication"`
	Precision  nulls.String `json:"precision" db:"precision"`
	Freeable   bool         `json:"freeable" db:"freeable"`
	Game       bool         `json:"game" db:"game"`
	Huntable   bool         `json:"huntable" db:"huntable"`
}

// localizeSpeciesHints translates status/indication/precision of the hint
// rows in place using preloaded translation maps; canonical (French) values
// are the fallback when no translation exists or lang is the base locale.
func localizeSpeciesHints(lang string, statusTr, indicationTr, precisionTr map[string]string, rows []speciesHint) {
	if lang == "" {
		return
	}
	for i := range rows {
		rows[i].Status = models.ResolveName(lang, rows[i].Status, statusTr, rows[i].ID)
		rows[i].Indication = models.ResolveName(lang, rows[i].Indication, indicationTr, rows[i].ID)
		if rows[i].Precision.Valid {
			rows[i].Precision.String = models.ResolveName(lang, rows[i].Precision.String, precisionTr, rows[i].ID)
		}
	}
}

// HintSpeciesDetails default implementation.
func HintSpeciesDetails(c buffalo.Context) error {
	s := []speciesHint{}

	q := c.Param("q")
	if len(q) == 0 {
		return c.Render(404, r.JSON(s))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query = tx.RawQuery(HINT_SPECIES_DETAILS, q)
	if err := query.All(&s); err != nil {
		return err
	}

	lang := currentLang(c)
	if lang != "" && len(s) > 0 {
		ids := make([]string, 0, len(s))
		for i := range s {
			ids = append(ids, s[i].ID)
		}
		statusTr := translateIDs(tx, "native_statuses", "status", lang, ids)
		indicationTr := translateIDs(tx, "native_statuses", "indication", lang, ids)
		precisionTr := translateIDs(tx, "native_statuses", "precision", lang, ids)
		localizeSpeciesHints(lang, statusTr, indicationTr, precisionTr, s)
	}

	return c.Render(200, r.JSON(s))
}
