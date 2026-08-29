package actions

import (
	"creaves/models"
	"fmt"
	"strconv"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// animalSearchParams holds the supported GET filters on /animals. All
// non-empty filters are combined with AND.
type animalSearchParams struct {
	Year          string
	AnimaltypeID  string
	Species       string
	EntryCauseID  string
	AnimalageID   string
	Ring          string
	OuttaketypeID string
}

// animalSearchParamsFrom extracts search filters from the request params.
func animalSearchParamsFrom(c buffalo.Context) animalSearchParams {
	return animalSearchParams{
		Year:          c.Param("year"),
		AnimaltypeID:  c.Param("animaltype_id"),
		Species:       c.Param("species"),
		EntryCauseID:  c.Param("entry_cause_id"),
		AnimalageID:   c.Param("animalage_id"),
		Ring:          c.Param("ring"),
		OuttaketypeID: c.Param("outtaketype_id"),
	}
}

// Any reports whether at least one filter is set.
func (p animalSearchParams) Any() bool {
	return p.Year != "" || p.AnimaltypeID != "" || p.Species != "" ||
		p.EntryCauseID != "" || p.AnimalageID != "" || p.Ring != "" ||
		p.OuttaketypeID != ""
}

// applyAnimalSearchFilters adds WHERE clauses for each non-empty filter.
// Subquery style is used for related-table filters (entry cause, outtake
// type) to avoid JOIN + DISTINCT row duplication. Outtakes whose type is
// flagged as an error are excluded from the outtaketype filter.
func applyAnimalSearchFilters(q *pop.Query, p animalSearchParams) (*pop.Query, error) {
	if p.Year != "" {
		y, err := strconv.Atoi(p.Year)
		if err != nil {
			return nil, fmt.Errorf("invalid year filter: %s", p.Year)
		}
		q = q.Where("animals.year = ?", y)
	}
	if p.AnimaltypeID != "" {
		q = q.Where("animals.animaltype_id = ?", p.AnimaltypeID)
	}
	if p.Species != "" {
		q = q.Where("animals.species = ?", p.Species)
	}
	if p.EntryCauseID != "" {
		q = q.Where("animals.discovery_id IN (SELECT id FROM discoveries WHERE entry_cause_id = ?)", p.EntryCauseID)
	}
	if p.AnimalageID != "" {
		q = q.Where("animals.animalage_id = ?", p.AnimalageID)
	}
	if p.Ring != "" {
		q = q.Where("animals.ring LIKE ?", "%"+p.Ring+"%")
	}
	if p.OuttaketypeID != "" {
		q = q.Where(`animals.outtake_id IN (
			SELECT o.id FROM outtakes o
			JOIN outtaketypes oo ON oo.id = o.outtaketype_id
			WHERE o.outtaketype_id = ? AND (oo.error = 0 OR oo.error IS NULL))`, p.OuttaketypeID)
	}
	return q, nil
}

// searchOption is one option in a filter-panel select, with sticky selection.
type searchOption struct {
	Value    string
	Label    string
	Selected bool
}

// blankSearchOption prepends an empty "all" option to the list.
func blankSearchOption(opts []searchOption, selected string) []searchOption {
	return append([]searchOption{{Value: "", Label: "", Selected: selected == ""}}, opts...)
}

// setupAnimalSearchContext loads the filter option lists (years, types, ages,
// entry causes, outtake types) into the context for the filter panel, with
// the current filter values pre-selected (sticky).
func setupAnimalSearchContext(c buffalo.Context, p animalSearchParams) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	lang := currentLang(c)

	years, err := listRegisterYears(c)
	if err != nil {
		return err
	}
	ys := make([]searchOption, 0, len(years)+1)
	for _, y := range years {
		ys = append(ys, searchOption{Value: y.Year, Label: y.Year, Selected: y.Year == p.Year})
	}
	c.Set("searchYears", blankSearchOption(ys, p.Year))

	at, err := animalTypes(c)
	if err != nil {
		return err
	}
	atIDs := make([]string, 0, len(*at))
	for _, t := range *at {
		atIDs = append(atIDs, t.ID.String())
	}
	atTr := translateIDs(tx, "animaltypes", "name", lang, atIDs)
	atOpts := make([]searchOption, 0, len(*at)+1)
	for _, t := range *at {
		atOpts = append(atOpts, searchOption{
			Value:    t.ID.String(),
			Label:    models.ResolveName(lang, t.Name, atTr, t.ID.String()),
			Selected: t.ID.String() == p.AnimaltypeID,
		})
	}
	c.Set("searchAnimalTypes", blankSearchOption(atOpts, p.AnimaltypeID))

	aa, err := animalages(c)
	if err != nil {
		return err
	}
	aaIDs := make([]string, 0, len(*aa))
	for _, t := range *aa {
		aaIDs = append(aaIDs, t.ID.String())
	}
	aaTr := translateIDs(tx, "animalages", "name", lang, aaIDs)
	aaOpts := make([]searchOption, 0, len(*aa)+1)
	for _, t := range *aa {
		aaOpts = append(aaOpts, searchOption{
			Value:    t.ID.String(),
			Label:    models.ResolveName(lang, t.Name, aaTr, t.ID.String()),
			Selected: t.ID.String() == p.AnimalageID,
		})
	}
	c.Set("searchAnimalages", blankSearchOption(aaOpts, p.AnimalageID))

	ec, err := entryCauses(c)
	if err != nil {
		return err
	}
	ecOpts := make([]searchOption, 0, len(*ec)+1)
	for _, t := range *ec {
		ecOpts = append(ecOpts, searchOption{
			Value:    t.ID,
			Label:    t.Fmt(true),
			Selected: t.ID == p.EntryCauseID,
		})
	}
	c.Set("searchEntryCauses", blankSearchOption(ecOpts, p.EntryCauseID))

	ot, err := outtakeTypes(c)
	if err != nil {
		return err
	}
	otIDs := make([]string, 0, len(*ot))
	for _, t := range *ot {
		otIDs = append(otIDs, t.ID.String())
	}
	otTr := translateIDs(tx, "outtaketypes", "name", lang, otIDs)
	otOpts := make([]searchOption, 0, len(*ot)+1)
	for _, t := range *ot {
		otOpts = append(otOpts, searchOption{
			Value:    t.ID.String(),
			Label:    models.ResolveName(lang, t.Name, otTr, t.ID.String()),
			Selected: t.ID.String() == p.OuttaketypeID,
		})
	}
	c.Set("searchOuttakeTypes", blankSearchOption(otOpts, p.OuttaketypeID))

	c.Set("search", p)
	c.Set("searchActive", p.Any())

	return nil
}

// animalCSVRow builds one CSV row for an enriched animal. Display values are
// resolved via the request-scoped tname/tspecies helpers (localized).
func animalCSVRow(c buffalo.Context, a models.Animal, entryCauses map[string]models.EntryCause) []string {
	tname, _ := c.Value("tname").(func(string, interface{}, interface{}) string)
	tspecies, _ := c.Value("tspecies").(func(interface{}) string)
	if tname == nil {
		tname = func(_ string, _ interface{}, base interface{}) string { return baseString(base) }
	}
	if tspecies == nil {
		tspecies = func(base interface{}) string { return baseString(base) }
	}

	intakeDate := ""
	if !a.Intake.Date.IsZero() {
		intakeDate = a.Intake.DateFormated()
	}

	entryCause := ""
	entryCauseDetail := ""
	if ec, ok := entryCauses[a.Discovery.EntryCauseID]; ok {
		entryCause = tname("entry_causes", ec.ID, ec.Cause)
		entryCauseDetail = ec.Detail
	}

	exitDate := ""
	exitReason := ""
	if a.Outtake != nil {
		exitDate = a.Outtake.DateFormated()
		exitReason = tname("outtaketypes", a.Outtake.Type.ID, a.Outtake.Type.Name)
	}

	return []string{
		fmt.Sprintf("%d", a.Year),
		fmt.Sprintf("%d", a.YearNumber),
		tname("animaltypes", a.Animaltype.ID, a.Animaltype.Name),
		tspecies(a.Species),
		a.Gender.String,
		tname("animalages", a.Animalage.ID, a.Animalage.Name),
		a.Ring.String,
		intakeDate,
		entryCause,
		entryCauseDetail,
		exitDate,
		exitReason,
		a.Zone.String,
		a.Cage.String,
	}
}

// animalCSVHeader returns the localized CSV header for the request language.
func animalCSVHeader(c buffalo.Context) []string {
	return []string{
		T.Translate(c, "animal.search.csv.year"),
		T.Translate(c, "animal.search.csv.number"),
		T.Translate(c, "animal.search.csv.type"),
		T.Translate(c, "animal.search.csv.species"),
		T.Translate(c, "animal.search.csv.gender"),
		T.Translate(c, "animal.search.csv.age"),
		T.Translate(c, "animal.search.csv.ring"),
		T.Translate(c, "animal.search.csv.intake_date"),
		T.Translate(c, "animal.search.csv.entry_cause"),
		T.Translate(c, "animal.search.csv.entry_cause_detail"),
		T.Translate(c, "animal.search.csv.exit_date"),
		T.Translate(c, "animal.search.csv.exit_reason"),
		T.Translate(c, "animal.search.csv.zone"),
		T.Translate(c, "animal.search.csv.cage"),
	}
}

// AnimalSearchExportCSV handles GET /animals/search/export.csv. It applies the
// same filters as AnimalsResource.List, without pagination, and streams the
// result as a CSV download via the shared CSV helper.
func AnimalSearchExportCSV(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	p := animalSearchParamsFrom(c)

	q := tx.Q()
	var err error
	if q, err = applyAnimalSearchFilters(q, p); err != nil {
		return err
	}

	animals := &models.Animals{}
	if err := q.Order("animals.year desc, animals.yearNumber desc").All(animals); err != nil {
		return err
	}

	if _, err := EnrichAnimalsOptimized(animals, c); err != nil {
		return err
	}

	// Entry causes lookup for the export columns
	ecs, err := entryCauses(c)
	if err != nil {
		return err
	}
	entryCausesMap := make(map[string]models.EntryCause, len(*ecs))
	for _, ec := range *ecs {
		entryCausesMap[ec.ID] = ec
	}

	rows := make([][]string, 0, len(*animals))
	for _, a := range *animals {
		rows = append(rows, animalCSVRow(c, a, entryCausesMap))
	}

	c.Logger().Infof("animal search export: %d rows", len(rows))
	return writeCSV(c, "animals.csv", animalCSVHeader(c), rows)
}
