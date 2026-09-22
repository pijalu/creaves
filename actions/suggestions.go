package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

func suggest(c buffalo.Context, table string, field string) error {
	s := []string{}

	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := "SELECT DISTINCT " + field + " FROM " + table
	var query *pop.Query
	if len(q) > 0 {
		query = tx.RawQuery(qroot+" WHERE "+field+" like ? ORDER BY 1 LIMIT 25", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot + " ORDER BY 1 LIMIT 25")
	}

	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsAnimalSpecies default implementation.
// When a non-base UI language is active, also matches against translated
// common names and returns the localized label for display. Submitted values
// are normalized back to the canonical (French) creaves_species value by
// resolveReferenceInput before storage (webhook contract stays stable).
func SuggestionsSpeciesType(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	q := strings.TrimSpace(c.Param("q"))
	lang := currentLang(c)
	var result struct {
		Species        string `db:"species" json:"species"`
		AnimaltypeID   string `db:"animaltype_id" json:"animaltype_id"`
		AnimaltypeName string `db:"animaltype_name" json:"animaltype_name"`
	}
	query := "SELECT s.creaves_species AS species, s.animaltype_id, t.name AS animaltype_name FROM species s LEFT JOIN animaltypes t ON t.id = s.animaltype_id"
	args := []interface{}{}
	if lang != "" {
		query += " LEFT JOIN translations tr ON tr.table_name = 'species' AND tr.field = 'creaves_species' AND tr.locale = ? AND tr.record_id = s.id"
		args = append(args, lang)
	}
	query += " WHERE (s.creaves_species = ?"
	args = append(args, q)
	if lang != "" {
		query += " OR tr.value = ?"
		args = append(args, q)
	}
	query += ") AND s.animaltype_id IS NOT NULL LIMIT 1"
	if err := tx.RawQuery(query, args...).First(&result); err != nil {
		return c.Render(http.StatusNotFound, r.JSON(map[string]string{"error": "species not found"}))
	}
	return c.Render(http.StatusOK, r.JSON(result))
}

func SuggestionsAnimalSpecies(c buffalo.Context) error {
	lang := currentLang(c)
	q := c.Param("q")
	animalTypeID := c.Param("animaltype_id")
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	where := []string{"1=1"}
	args := []interface{}{}
	if animalTypeID != "" {
		where = append(where, "s.animaltype_id = ?")
		args = append(args, animalTypeID)
	}
	if q != "" {
		if lang != "" {
			where = append(where, "(s.creaves_species LIKE ? OR t.value LIKE ?)")
			args = append(args, "%"+q+"%", "%"+q+"%")
		} else {
			where = append(where, "s.creaves_species LIKE ?")
			args = append(args, "%"+q+"%")
		}
	}
	query := "SELECT DISTINCT s.creaves_species FROM species s"
	if lang != "" {
		query += " LEFT JOIN translations t ON t.table_name = 'species' AND t.field = 'creaves_species' AND t.locale = ? AND t.record_id = s.id"
		args = append([]interface{}{lang}, args...)
	}
	query += " WHERE " + strings.Join(where, " AND ") + " ORDER BY 1 LIMIT 25"
	var s []string
	if err := tx.RawQuery(query, args...).All(&s); err != nil {
		return err
	}
	return c.Render(200, r.JSON(localizeSuggestions(c, "species", "creaves_species", s)))
}

// SuggestionsDiscoveryLocation default implementation.
func SuggestionsDiscoveryLocation(c buffalo.Context) error {
	return suggest(c, "discoveries", "location")
}

// SuggestionsOuttakeLocation default implementation.
func SuggestionsOuttakeLocation(c buffalo.Context) error {
	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := `SELECT CONCAT(postal_code,"_",locality) FROM localities`
	var query *pop.Query
	if len(q) > 0 {
		query = tx.RawQuery(qroot+` WHERE CONCAT(postal_code,"_",locality) like ? ORDER BY 1 LIMIT 25`, "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot + ` ORDER BY 1 LIMIT 25`)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsCorpseDestination returns the DISTINCT previously used corpse
// destination values from outtakes (issue #149) for the autocomplete of the
// corpse register marking form.
func SuggestionsCorpseDestination(c buffalo.Context) error {
	return suggest(c, "outtakes", "corpse_destination")
}

// Discoverer city values in the production database sometimes merge the zip
// code into the city ("67000 Strasbourg"). Stored rows are never rewritten
// (production data); instead the search endpoints clean what they return so
// searching still matches and autocomplete fill produces correct values
// (bugs.md #9).
var (
	// leadingZipInCityRe matches "67000 Strasbourg", "B-6700 Strasbourg"
	// and the underscore variant "4280_Avin" found in production data.
	leadingZipInCityRe = regexp.MustCompile(`(?i)^(?:[a-z]{1,3}-)?(\d{4,6})[_\s]+(\S.*)$`)
	// trailingZipInCityRe matches "Strasbourg 67000" / "Strasbourg B-6700".
	trailingZipInCityRe = regexp.MustCompile(`^(.+?)[_\s]+(?:[a-zA-Z]{1,3}-)?(\d{4,6})$`)
)

// splitPostalCity separates a postal code accidentally merged into the city
// value. The extracted zip is adopted as the postal code when that field is
// empty; the returned city is always the clean city name.
func splitPostalCity(postal, city string) (string, string) {
	city = strings.TrimSpace(city)
	postal = strings.TrimSpace(postal)
	if m := leadingZipInCityRe.FindStringSubmatch(city); m != nil {
		if postal == "" {
			postal = m[1]
		}
		return postal, strings.TrimSpace(m[2])
	}
	if m := trailingZipInCityRe.FindStringSubmatch(city); m != nil {
		if postal == "" {
			postal = m[2]
		}
		return postal, strings.TrimSpace(m[1])
	}
	return postal, city
}

// SuggestionsDiscovererCity returns the distinct discoverer city values for
// the city autocomplete on the discoverer form. The LIKE filter runs on the
// raw stored values — so a search for a zip still finds the merged entries —
// but the returned suggestions are cleaned and de-duplicated so filling the
// form yields a correct city (bugs.md #9).
func SuggestionsDiscovererCity(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	q := c.Param("q")
	query := "SELECT DISTINCT city FROM discoverers"
	var args []interface{}
	if len(q) > 0 {
		query += " WHERE city LIKE ?"
		args = append(args, "%"+q+"%")
	}
	query += " ORDER BY 1 LIMIT 25"

	raw := []string{}
	if err := tx.RawQuery(query, args...).All(&raw); err != nil {
		return err
	}

	seen := map[string]bool{}
	s := make([]string, 0, len(raw))
	for _, v := range raw {
		_, city := splitPostalCity("", v)
		if city == "" || seen[city] {
			continue
		}
		seen[city] = true
		s = append(s, city)
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsDiscovererCountry default implementation.
func SuggestionsDiscovererCountry(c buffalo.Context) error {
	return suggest(c, "discoverers", "country")
}

// SuggestionsDiscovererCountry default implementation.
func SuggestionsPostalCode(c buffalo.Context) error {
	return suggest(c, "localities", "postal_code")
}

// SuggestionsOuttakeLocation default implementation.
func SuggestionsLocality(c buffalo.Context) error {
	z := c.Param("z") // zip
	l := c.Param("l") // locality

	ret := c.Param("r") // return

	var field string
	switch ret {
	case "postal_code":
		field = "postal_code"
	case "locality":
		field = "locality"
	default:
		return fmt.Errorf("unexpected request for %s", ret)
	}

	s := []string{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := fmt.Sprintf(`SELECT distinct %s FROM localities WHERE 1=1`, field)
	args := []interface{}{}

	// Add postal code
	if len(z) > 0 {
		qroot += ` AND postal_code LIKE ?`
		args = append(args, "%"+z+"%")
	}

	// Add zip code
	if len(l) > 0 {
		qroot += ` AND locality LIKE ?`
		args = append(args, "%"+l+"%")
	}
	qroot += " ORDER BY 1 LIMIT 10"
	c.Logger().Debugf("Query: %s - params: %v", qroot, args)
	query := tx.RawQuery(qroot, args...)

	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// Suggest discovered
func SuggestionsDiscoverer(c buffalo.Context) error {
	f := c.Param("f") // first name
	l := c.Param("l") // last name
	a := c.Param("a") // address

	ret := c.Param("r") // return

	s := []string{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var field string

	switch ret {
	case "firstname":
		field = "firstname"
	case "lastname":
		field = "lastname"
	case "address":
		field = "address"
	default:
		return fmt.Errorf("unexpected request for %s", ret)
	}

	qroot := fmt.Sprintf(`SELECT DISTINCT %s FROM discoverers WHERE 1=1 `, field)
	args := []interface{}{}

	// Add first name
	if len(f) > 0 {
		qroot += ` AND firstname like ?`
		args = append(args, "%"+f+"%")
	}

	// Add last name
	if len(l) > 0 {
		qroot += ` AND lastname LIKE ?`
		args = append(args, "%"+l+"%")
	}

	// add Address
	if len(a) > 0 {
		qroot += ` AND address LIKE ?`
		args = append(args, "%"+a+"%")
	}

	qroot += " ORDER BY 1 LIMIT 25"
	c.Logger().Debugf("Query: %s - params: %v", qroot, args)
	query := tx.RawQuery(qroot, args...)

	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsAnimaltypeDefault returns the default species of a single animal
// type as a one-element JSON array (empty array when the type has none or does
// not exist). Used by the reception wizard to pre-fill the species field only
// when the selected type actually defines a default species — see issue #199:
// previously the first species suggestion of the type was copied, silently
// filling a species for types without any default.
func SuggestionsAnimaltypeDefault(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	s := []string{}
	animalTypeID := strings.TrimSpace(c.Param("animaltype_id"))
	if animalTypeID != "" {
		if err := tx.RawQuery("SELECT default_species FROM animaltypes WHERE id = ? AND default_species IS NOT NULL AND default_species <> '' LIMIT 1", animalTypeID).All(&s); err != nil {
			return err
		}
	}
	return c.Render(200, r.JSON(localizeSuggestions(c, "species", "creaves_species", s)))
}

// SuggestionsAnimalTypeDefaultSpecies default implementation.
// Suggests species names from animaltypes.default_species; when a non-base UI
// language is active, matches translated species names and returns localized
// labels.
func SuggestionsAnimalTypeDefaultSpecies(c buffalo.Context) error {
	q := c.Param("q")
	lang := currentLang(c)

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := "SELECT distinct default_species FROM animaltypes WHERE default_species is NOT NULL AND default_species <> '' "
	var query *pop.Query
	if len(q) > 0 {
		if lang != "" {
			// Match the localized species label too (translations on species.creaves_species)
			query = tx.RawQuery(qroot+" AND (default_species like ? OR default_species IN (SELECT fr.value FROM species s JOIN translations fr ON fr.table_name = 'species' AND fr.record_id = s.id AND fr.field = 'creaves_species' AND fr.locale = 'fr' JOIN translations tr ON tr.table_name = fr.table_name AND tr.record_id = fr.record_id AND tr.field = fr.field AND tr.locale = ? WHERE s.creaves_species = fr.value AND tr.value LIKE ?)) ORDER BY 1 LIMIT 25", "%"+q+"%", lang, "%"+q+"%")
		} else {
			query = tx.RawQuery(qroot+" AND default_species like ? ORDER BY 1 LIMIT 25", "%"+q+"%")
		}
	} else {
		query = tx.RawQuery(qroot + " ORDER BY 1 LIMIT 25")
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(localizeSuggestions(c, "species", "creaves_species", s)))
}

// SuggestionsTreatmentDrug default implementation.
// When a non-base UI language is active, matches translated drug names and
// returns localized labels (canonical storage handled by resolveReferenceInput).
func SuggestionsTreatmentDrug(c buffalo.Context) error {
	q := c.Param("q")
	at := c.Param("at")
	//w := c.Param("w")
	lang := currentLang(c)

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := `
		SELECT d.name 
		FROM drugs d, 
		     dosages s 
		WHERE d.ID = s.drug_id 
		  AND s.animaltype_id = ?`

	if len(q) > 0 && lang != "" {
		query = tx.RawQuery(qroot+" AND (d.Name like ? OR d.Name IN (SELECT fr.value FROM translations fr WHERE fr.table_name = 'drugs' AND fr.field = 'name' AND fr.locale = 'fr' AND EXISTS (SELECT 1 FROM translations tr WHERE tr.table_name = 'drugs' AND tr.record_id = fr.record_id AND tr.field = 'name' AND tr.locale = ? AND tr.value LIKE ?))) ORDER BY 1 LIMIT 25", at, "%"+q+"%", lang, "%"+q+"%")
	} else if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND d.Name like ? ORDER BY 1 LIMIT 25", at, "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot+" ORDER BY 1 LIMIT 25", at)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(localizeSuggestions(c, "drugs", "name", s)))
}

// SuggestionsDrugRemark returns the remark (description) of a drug by exact
// name. Used by the treatment form to prefill the remarks field when a drug
// is selected (issue #73). Missing or empty remarks yield an empty string so
// the client can safely no-op.
func SuggestionsDrugRemark(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	drug := &models.Drug{}
	if err := tx.Where("name = ?", c.Param("name")).First(drug); err != nil {
		return c.Render(http.StatusOK, r.JSON(map[string]string{"description": ""}))
	}
	return c.Render(http.StatusOK, r.JSON(map[string]string{"description": drug.Description.String}))
}

// SuggestionsFeedingGuide returns the diet text ("régime alimentaire") for a
// species at a life stage (issue #145). Missing guides yield an empty string
// so the client can show a graceful "no guide" state. Species is matched
// exactly (canonical name), stage must be one of the canonical stages.
func SuggestionsFeedingGuide(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	guide := &models.FeedingGuide{}
	if err := tx.Where("species_name = ? AND stage = ?", c.Param("species"), c.Param("stage")).First(guide); err != nil {
		return c.Render(http.StatusOK, r.JSON(map[string]string{"text": ""}))
	}
	return c.Render(http.StatusOK, r.JSON(map[string]string{"text": guide.Text}))
}

// SuggestionsTreatmentDrug default implementation.
func SuggestionsTreatmentDrugDosage(c buffalo.Context) error {
	result := []string{}

	q := c.Param("q")
	at := c.Param("at")
	w, err := strconv.ParseFloat(c.Param("w"), 64)
	if err != nil {
		c.Logger().Debugf("Failed to convert weight to integer:%v", err)
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	c.Logger().Debugf("Dosage for: %v %v %v", q, at, w)

	if len(q) == 0 {
		c.Logger().Debugf("No medication name provided")
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	d := &models.Dosage{}
	var query *pop.Query
	qroot := `
		SELECT s.* 
		FROM drugs d, 
		     dosages s 
		WHERE d.Name = ?
		  AND d.ID = s.drug_id
		  AND s.animaltype_id = ?`

	query = tx.RawQuery(qroot, q, at)

	if err := query.First(d); err != nil {
		c.Logger().Debugf("Dosage lookup failed: %v", err)
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	//c.Logger().Debugf("Loaded mediaction/dosage: %v", d)

	if !d.DosagePerGrams.Valid {
		c.Logger().Debugf("No dosage for drug")
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	c.Logger().Debugf("Dosage: %v * %v", w, d.DosagePerGrams.Float64)
	ds := w * d.DosagePerGrams.Float64

	result = append(result, fmt.Sprintf("%.2f %s", ds, d.DosagePerGramsUnit.String))

	return c.Render(200, r.JSON(result))
}

// SuggestionsAnimalInCare - specific implementation to only account in care animal (no outtake).
func SuggestionsAnimalInCare(c buffalo.Context) error {
	results := []struct {
		Year       string `json:"Year" db:"Year"`
		YearNumber string `json:"YearNumber" db:"YearNumber"`
	}{}

	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := "SELECT Year, YearNumber FROM animals WHERE outtake_id IS null "
	var query *pop.Query
	if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND YearNumber like ? ORDER BY Year, YearNumber LIMIT 25", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot + " ORDER BY Year, YearNumber LIMIT 25")
	}

	if err := query.All(&results); err != nil {
		return err
	}

	// return a series of strings
	s := []string{}
	for _, result := range results {
		s = append(s, fmt.Sprintf("%s/%s", result.YearNumber, result.Year[2:]))
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsCagesAnimalInCare - specific implementation to only account for cages with animal (no outtake).
func SuggestionsCageWithAnimalInCare(c buffalo.Context) error {
	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	qroot := "SELECT DISTINCT Cage FROM animals WHERE outtake_id IS null and Cage is not null"
	var query *pop.Query
	if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND Cage like ? ORDER BY 1 LIMIT 25", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot + " ORDER BY 1 LIMIT 25")
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}
