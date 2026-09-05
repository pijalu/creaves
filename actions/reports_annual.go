package actions

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/nicksnyder/go-i18n/i18n"
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

// annualCategorySources maps a statistics section to the reference table and
// column its category values come from. Only fixed, hard-coded table/field
// names are allowed through the localizer.
var annualCategorySources = map[string]struct{ table, field string }{
	"species":            {"species", "creaves_species"},
	"class":              {"species", "class"},
	"agw_group":          {"species", "agw_group"},
	"subsidies_group":    {"subside_groups", "group"},
	"native_status":      {"native_statuses", "status"},
	"entry_age":          {"animalages", "name"},
	"outtake_type":       {"outtaketypes", "name"},
	"entry_cause":        {"entry_causes", "cause"},
	"entry_cause_nature": {"entry_causes", "nature"},
}

// loadAnnualRefTranslations returns canonical base value -> localized value
// for one (table, field, lang), joining translations by record id. Table and
// field must come from the fixed annualCategorySources map. Returns an empty
// map on any error (categories stay canonical).
func loadAnnualRefTranslations(tx *pop.Connection, table, field, lang string) map[string]string {
	out := map[string]string{}
	var rows []struct {
		Base  string `db:"base"`
		Value string `db:"value"`
	}
	q := "SELECT src.`" + field + "` AS base, tr.value AS value" +
		" FROM `" + table + "` src" +
		" JOIN translations tr ON tr.table_name = ? AND tr.record_id = src.id AND tr.field = ? AND tr.locale = ? AND tr.value <> ''" +
		" WHERE src.`" + field + "` IS NOT NULL AND src.`" + field + "` <> ''"
	if err := tx.RawQuery(q, table, field, lang).All(&rows); err != nil {
		return out
	}
	for _, r := range rows {
		out[r.Base] = r.Value
	}
	return out
}

// localizeAnnualSections rewrites category display values of the annual
// statistics tables into the request language. Reference-derived categories
// (species, groups, ages, outtake types, entry causes) are translated through
// the translations table; fixed SQL literals (Unknown/Dead/Neutral/Alive/
// Released) through the reports locale files; entry_cause_detail composite
// values are localized part by part. Canonical values are kept as fallback so
// the report never loses rows.
func localizeAnnualSections(c buffalo.Context, sections []annualStatSection) {
	lang := currentLang(c)
	if lang == "" {
		return
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return
	}

	refMaps := map[string]map[string]string{}
	ref := func(table, field string) map[string]string {
		k := table + "\x00" + field
		if m, ok := refMaps[k]; ok {
			return m
		}
		m := loadAnnualRefTranslations(tx, table, field, lang)
		refMaps[k] = m
		return m
	}
	literals := map[string]string{}
	lit := func(value, key string) string {
		if v, ok := literals[value]; ok {
			return v
		}
		v := key
		// T.Translate looks up the per-request translate func that the i18n
		// middleware stores on the context; in tests the middleware never ran,
		// so the lookup would panic. Fall back to the raw key then.
		if tf, ok := c.Value("T").(i18n.TranslateFunc); ok {
			v = tf(key)
		}
		literals[value] = v
		return v
	}

	for si := range sections {
		sec := &sections[si]
		for ri := range sec.Rows {
			cat := sec.Rows[ri].Category
			switch {
			case cat == "Unknown":
				sec.Rows[ri].Category = lit(cat, "reports.annual.unknown")
			case sec.ID == "outtake_rating":
				switch cat {
				case "Dead":
					sec.Rows[ri].Category = lit(cat, "reports.annual.rating.dead")
				case "Neutral":
					sec.Rows[ri].Category = lit(cat, "reports.annual.rating.neutral")
				case "Alive":
					sec.Rows[ri].Category = lit(cat, "reports.annual.rating.alive")
				}
			case sec.ID == "outtake_dead_released":
				switch cat {
				case "Dead":
					sec.Rows[ri].Category = lit(cat, "reports.annual.rating.dead")
				case "Released":
					sec.Rows[ri].Category = lit(cat, "reports.annual.outtake.released")
				}
			case sec.ID == "entry_cause_detail":
				// Value is CONCAT_WS(' / ', nature, cause, detail) — localize
				// each non-empty part, trying nature, then cause, then detail.
				parts := strings.Split(cat, " / ")
				for i, part := range parts {
					for _, field := range []string{"nature", "cause", "detail"} {
						if v := ref("entry_causes", field)[part]; v != "" {
							parts[i] = v
							break
						}
					}
				}
				sec.Rows[ri].Category = strings.Join(parts, " / ")
			default:
				if src, ok := annualCategorySources[sec.ID]; ok {
					if v := ref(src.table, src.field)[cat]; v != "" {
						sec.Rows[ri].Category = v
					}
				}
			}
		}
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
	localizeAnnualSections(c, sections)
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
	localizeAnnualSections(c, sections)

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
