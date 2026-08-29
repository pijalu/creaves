package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// Annual statistics report (plan §5, Creaves side).
//
// All tables are based on the animals table for a single year, counting
// DISTINCT animals (COUNT(DISTINCT a.id)) so that single-row LEFT JOINs never
// multiply rows. Animals whose outtake type is flagged error=1 are excluded
// everywhere (oo.error exclusion). NULL categories are grouped into an
// "Unknown" bucket. Each table shows the count and the percentage of T (the
// table total) with 1 decimal, plus a total row equal to T.

// annualStatRow is one category line of a statistics table.
type annualStatRow struct {
	Category string `db:"category"`
	Count    int    `db:"n"`
	Percent  string
}

// annualStatSection is one of the 12 statistics tables.
type annualStatSection struct {
	ID    string // localization key suffix + anchor
	Rows  []annualStatRow
	Total int
}

// annualStatQuery couples a grouped query (category, n) with the matching
// total query (T) for the same FROM/WHERE.
type annualStatQuery struct {
	id        string
	grouped   string
	total     string
	orderDesc bool // true: query orders by count desc; false: explicit category order kept
}

// errorFilter excludes animals whose outtake type is an error type.
const annualErrorFilter = `(oo.error = 0 OR oo.error IS NULL)`

// Shared join fragments. Only single-row relations are joined (outtake via
// a.outtake_id, intake via a.intake_id, discovery via a.discovery_id) so
// COUNT(DISTINCT a.id) stays exact.
const (
	annualJoinOuttake = `
		LEFT JOIN outtakes AS o ON a.outtake_id = o.id
		LEFT JOIN outtaketypes AS oo ON o.outtaketype_id = oo.id`
	annualJoinSpecies = `
		LEFT JOIN species AS sp ON a.species = sp.creaves_species`
	annualJoinDiscovery = `
		LEFT JOIN discoveries AS d ON a.discovery_id = d.id
		LEFT JOIN entry_causes AS ec ON d.entry_cause_id = ec.id`
	annualJoinOuttakeInner = `
		INNER JOIN outtakes AS o ON a.outtake_id = o.id
		INNER JOIN outtaketypes AS oo ON o.outtaketype_id = oo.id`
)

func annualStatQueries() []annualStatQuery {
	base := `FROM animals AS a`
	return []annualStatQuery{
		{
			id: "species",
			grouped: `SELECT COALESCE(NULLIF(sp.creaves_species, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC LIMIT 20`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "class",
			grouped: `SELECT COALESCE(NULLIF(sp.class, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "agw_group",
			grouped: `SELECT COALESCE(NULLIF(sp.agw_group, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "subsidies_group",
			grouped: `SELECT COALESCE(NULLIF(sg.` + "`group`" + `, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinSpecies + `
				LEFT JOIN subside_groups AS sg ON sp.subside_group = sg.id` + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "native_status",
			grouped: `SELECT COALESCE(NULLIF(ns.status, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinSpecies + `
				LEFT JOIN native_statuses AS ns ON sp.native_status = ns.id` + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinSpecies + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "entry_age",
			grouped: `SELECT COALESCE(NULLIF(aa.name, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + `
				LEFT JOIN animalages AS aa ON a.animalage_id = aa.id` + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "outtake_type",
			grouped: `SELECT COALESCE(NULLIF(oo.name, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "outtake_rating",
			grouped: `SELECT CASE oo.rating
				WHEN -1 THEN 'Dead'
				WHEN 0 THEN 'Neutral'
				WHEN 1 THEN 'Alive'
				ELSE 'Unknown' END AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY MIN(oo.rating) ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter,
		},
		{
			id: "outtake_dead_released",
			grouped: `SELECT CASE WHEN oo.dead = 1 THEN 'Dead' ELSE 'Released' END AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY MIN(oo.dead) DESC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinOuttakeInner + `
				WHERE a.year = ? AND ` + annualErrorFilter,
		},
		{
			id: "entry_cause",
			grouped: `SELECT COALESCE(NULLIF(ec.cause, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "entry_cause_detail",
			grouped: `SELECT COALESCE(NULLIF(CONCAT_WS(' / ', NULLIF(ec.nature, ''), NULLIF(ec.cause, ''), NULLIF(ec.detail, '')), ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
		{
			id: "entry_cause_nature",
			grouped: `SELECT COALESCE(NULLIF(ec.nature, ''), 'Unknown') AS category, COUNT(DISTINCT a.id) AS n ` +
				base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter + `
				GROUP BY 1 ORDER BY n DESC, category ASC`,
			total: `SELECT COUNT(DISTINCT a.id) ` + base + annualJoinDiscovery + annualJoinOuttake + `
				WHERE a.year = ? AND ` + annualErrorFilter,
			orderDesc: true,
		},
	}
}

// annualStatPercent formats count as a percentage of total with 1 decimal.
func annualStatPercent(count, total int) string {
	if total == 0 {
		return "0.0"
	}
	return fmt.Sprintf("%.1f", float64(count)*100.0/float64(total))
}

// runAnnualStatQuery executes one grouped query + its total query for year.
func runAnnualStatQuery(tx *pop.Connection, q annualStatQuery, year string) (annualStatSection, error) {
	sec := annualStatSection{ID: q.id, Rows: []annualStatRow{}}
	if err := tx.RawQuery(q.grouped, year).All(&sec.Rows); err != nil {
		return sec, fmt.Errorf("annual stat %s: %w", q.id, err)
	}
	var total int
	if err := tx.RawQuery(q.total, year).First(&total); err != nil {
		return sec, fmt.Errorf("annual stat %s total: %w", q.id, err)
	}
	// For grouped tables the total T is the number of distinct animals in
	// scope; for the top-20 species table the sum of the (limited) rows can
	// be lower than T, which is expected.
	sec.Total = total
	for i := range sec.Rows {
		sec.Rows[i].Percent = annualStatPercent(sec.Rows[i].Count, total)
	}
	return sec, nil
}

// runAnnualStats computes all 12 statistics tables for year.
func runAnnualStats(tx *pop.Connection, year string) ([]annualStatSection, error) {
	queries := annualStatQueries()
	sections := make([]annualStatSection, 0, len(queries))
	for _, q := range queries {
		sec, err := runAnnualStatQuery(tx, q, year)
		if err != nil {
			return nil, err
		}
		sections = append(sections, sec)
	}
	return sections, nil
}

// selectAnnualYear resolves the requested year (or the latest register year by
// default) and flags the selected entry in the dropdown list.
func selectAnnualYear(c buffalo.Context) ([]registerYear, string, error) {
	years, err := listRegisterYears(c)
	if err != nil {
		return nil, "", err
	}
	if len(years) == 0 {
		return years, "", nil
	}
	y := c.Param("year")
	selected := ""
	for i := range years {
		years[i].Selected = years[i].Year == y
		if years[i].Selected {
			selected = years[i].Year
		}
	}
	if selected == "" {
		years[len(years)-1].Selected = true
		selected = years[len(years)-1].Year
	}
	return years, selected, nil
}

// ReportsAnnualIndex handles GET /reports/annual?year=YYYY.
func ReportsAnnualIndex(c buffalo.Context) error {
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

	sections := []annualStatSection{}
	if selectedYear != "" {
		if sections, err = runAnnualStats(tx, selectedYear); err != nil {
			return err
		}
	}
	c.Set("sections", sections)

	return c.Render(http.StatusOK, r.HTML("reports/annual.plush.html"))
}

// ReportsAnnualExportCSV handles GET /reports/annual/export.csv?year=YYYY.
// Single multi-section CSV: Year, Section, Category, Count, Percent.
func ReportsAnnualExportCSV(c buffalo.Context) error {
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

	sections, err := runAnnualStats(tx, selectedYear)
	if err != nil {
		return err
	}

	header := []string{
		T.Translate(c, "reports.annual.csv.year"),
		T.Translate(c, "reports.annual.csv.section"),
		T.Translate(c, "reports.annual.csv.category"),
		T.Translate(c, "reports.annual.csv.count"),
		T.Translate(c, "reports.annual.csv.percent"),
	}
	rows := [][]string{}
	for _, sec := range sections {
		sectionName := T.Translate(c, "reports.annual.section."+sec.ID)
		for _, row := range sec.Rows {
			rows = append(rows, []string{
				selectedYear,
				sectionName,
				row.Category,
				fmt.Sprintf("%d", row.Count),
				row.Percent,
			})
		}
		rows = append(rows, []string{
			selectedYear,
			sectionName,
			T.Translate(c, "reports.annual.total"),
			fmt.Sprintf("%d", sec.Total),
			"100.0",
		})
	}

	return writeCSV(c, fmt.Sprintf("reports-annual-%s.csv", selectedYear), header, rows)
}
