package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Corpse register (issue #149): lists animals whose outtake type has
// Dead=true (i.e. the outtake produced a corpse) for a given year, with the
// corpse destination tracking columns. Mirrors the registertable layout.

// corpseRow is one line of the corpse register. Raw-query mapped: yearNumber
// keeps the animals table's camelCase column name.
type corpseRow struct {
	AnimalID   int       `db:"animal_id"`
	Year       int       `db:"year"`
	YearNumber int       `db:"yearNumber"`
	Species    string    `db:"species"`
	IntakeDate time.Time `db:"intake_date"`
	OuttakeID  uuid.UUID `db:"outtake_id"`
	DeathDate  time.Time `db:"death_date"`

	CorpseDestination   nulls.String `db:"corpse_destination"`
	CorpseDestinationAt nulls.Time   `db:"corpse_destination_at"`
	CorpseDestinationBy nulls.String `db:"corpse_destination_by"`
}

const SQL_CORPSE_ROWS = `
select a.id as animal_id, a.year, a.yearNumber, a.species,
       i.date as intake_date, o.id as outtake_id, o.date as death_date,
       o.corpse_destination, o.corpse_destination_at, u.login as corpse_destination_by
from animals a
join outtakes o on a.outtake_id = o.id
join outtaketypes ot on o.outtaketype_id = ot.id and ot.dead = 1
join intakes i on a.intake_id = i.id
left join users u on o.corpse_destination_by_id = u.id
where a.year = ?
order by a.yearNumber asc`

func listCorpseRows(tx *pop.Connection, year string) ([]corpseRow, error) {
	rows := []corpseRow{}
	if err := tx.RawQuery(SQL_CORPSE_ROWS, year).All(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ReportsCorpsesIndex handles GET /reports/corpses?year=YYYY.
func ReportsCorpsesIndex(c buffalo.Context) error {
	years, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	c.Set("years", years)
	c.Set("selectedYear", selectedYear)

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows := []corpseRow{}
	if selectedYear != "" {
		if rows, err = listCorpseRows(tx, selectedYear); err != nil {
			return err
		}
	}
	c.Set("corpses", rows)
	c.Set("isAdmin", GetCurrentUser(c).Admin)

	return c.Render(http.StatusOK, r.HTML("reports/corpses.plush.html"))
}

// ReportsCorpsesExportCSV handles GET /reports/corpses/export.csv?year=YYYY.
func ReportsCorpsesExportCSV(c buffalo.Context) error {
	_, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	if selectedYear == "" {
		return fmt.Errorf("year not provided")
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows, err := listCorpseRows(tx, selectedYear)
	if err != nil {
		return err
	}

	header := []string{
		T.Translate(c, "reports.corpses.csv.number"),
		T.Translate(c, "reports.corpses.csv.species"),
		T.Translate(c, "reports.corpses.csv.entry"),
		T.Translate(c, "reports.corpses.csv.death_date"),
		T.Translate(c, "reports.corpses.csv.destination"),
		T.Translate(c, "reports.corpses.csv.destination_at"),
		T.Translate(c, "reports.corpses.csv.destination_by"),
	}
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		destAt := ""
		if row.CorpseDestinationAt.Valid {
			destAt = row.CorpseDestinationAt.Time.Format("02/01/2006 15:04")
		}
		records = append(records, []string{
			fmt.Sprintf("%d", row.YearNumber),
			row.Species,
			row.IntakeDate.Format("02/01/2006"),
			row.DeathDate.Format("02/01/2006"),
			row.CorpseDestination.String,
			destAt,
			row.CorpseDestinationBy.String,
		})
	}

	return writeCSV(c, fmt.Sprintf("reports-corpses-%s.csv", selectedYear), header, records)
}

// ReportsCorpsesMark handles POST /reports/corpses/mark.
// Params: outtake_ids (repeated), destination, destination_at (optional,
// defaults to now). The recording user is always taken from the session —
// never from the form. Outtakes whose type is not Dead=true are ignored.
func ReportsCorpsesMark(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	user := GetCurrentUser(c)
	if user == nil {
		return c.Render(http.StatusForbidden, r.String("no current user"))
	}

	ids := c.Request().Form["outtake_ids"]
	destination := c.Param("destination")
	if len(ids) == 0 {
		return c.Render(http.StatusBadRequest, r.String("no outtake selected"))
	}

	markedAt := time.Now()
	if raw := c.Param("destination_at"); raw != "" {
		for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
				markedAt = t
				break
			}
		}
	}

	// Only mark outtakes whose type is Dead=true; silently ignore the rest.
	eligible := []models.Outtake{}
	if err := tx.RawQuery(`
		select o.* from outtakes o
		join outtaketypes ot on o.outtaketype_id = ot.id and ot.dead = 1
		where o.id in (?)`, ids).All(&eligible); err != nil {
		return err
	}

	marked := 0
	for i := range eligible {
		o := &eligible[i]
		o.CorpseDestination = nulls.NewString(destination)
		o.CorpseDestinationAt = nulls.NewTime(markedAt)
		o.CorpseDestinationByID = nulls.NewUUID(user.ID)
		if err := tx.Save(o); err != nil {
			return err
		}
		marked++
	}

	c.Flash().Add("success", fmt.Sprintf("%d corpse(s) marked", marked))
	// Redirect back to the originating page when a local "back" param is
	// given (e.g. the animal sheet's outtake tab, issue #199-6); otherwise
	// fall back to the corpse register.
	if back := c.Param("back"); safeRedirectTarget(back) != "/" {
		return c.Redirect(302, safeRedirectTarget(back))
	}
	return c.Redirect(302, "/reports/corpses?year=%s", c.Param("year"))
}

// ReportsCorpsesUnmark handles POST /reports/corpses/unmark — admin only.
// Clears destination/date/recorder on the selected dead-type outtakes so a
// mistaken mark can be corrected.
func ReportsCorpsesUnmark(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	ids := c.Request().Form["outtake_ids"]
	if len(ids) == 0 {
		return c.Render(http.StatusBadRequest, r.String("no outtake selected"))
	}

	// Only unmark outtakes whose type is Dead=true; silently ignore the rest.
	eligible := []models.Outtake{}
	if err := tx.RawQuery(`
		select o.* from outtakes o
		join outtaketypes ot on o.outtaketype_id = ot.id and ot.dead = 1
		where o.id in (?)`, ids).All(&eligible); err != nil {
		return err
	}

	unmarked := 0
	for i := range eligible {
		o := &eligible[i]
		o.CorpseDestination = nulls.String{}
		o.CorpseDestinationAt = nulls.Time{}
		o.CorpseDestinationByID = nulls.UUID{}
		if err := tx.Save(o); err != nil {
			return err
		}
		unmarked++
	}

	c.Flash().Add("success", fmt.Sprintf("%d corpse(s) unmarked", unmarked))
	if back := c.Param("back"); safeRedirectTarget(back) != "/" {
		return c.Redirect(302, safeRedirectTarget(back))
	}
	return c.Redirect(302, "/reports/corpses?year=%s", c.Param("year"))
}
