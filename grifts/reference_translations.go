package grifts

import (
	"fmt"
	"strings"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Reference-data translations for tables seeded from Go structs / CSV rather
// than the startup dump artifacts (translations_*.sql cover only the 7 dump
// tables). Canonical (base) column values stay French; this data provides the
// en-US / de / nl values. Locale order in [3]string values is fixed by
// refLocales. Applied idempotently as part of `db:seed`: existing translation
// rows (per record, or per canonical value when record IDs drifted) are
// preserved; only missing rows are inserted. Locale "fr" is intentionally
// absent — the base column is the canonical French value.

var refLocales = [3]string{"en-US", "de", "nl"}

// refFieldTr maps a field name to its [en-US, de, nl] values.
type refFieldTr map[string][3]string

// referenceTranslations: table -> record ID -> field translations.
// Record IDs match the seed sources (create_entry_cause.csv "ID" column,
// native_statuses "NS*"', subside_groups "SG*").
var referenceTranslations = map[string]map[string]refFieldTr{
	"entry_causes": {
		"1.1": {
			"cause":      {"Undetermined", "Unbestimmt", "Onbepaald"},
			"detail":     {"Undetermined", "Unbestimmt", "Onbepaald"},
			"nature":     {"Unidentified", "Nicht identifiziert", "Niet geïdentificeerd"},
			"indication": {"left in front of the center, no finder, no information", "vor dem Zentrum abgegeben, kein Finder, keine Informationen", "voor het centrum achtergelaten, geen vinder, geen info"},
		},
		"2.1": {
			"cause":      {"Found in an unusual place, at an unusual time, or in an unusual condition", "An einem ungewöhnlichen Ort, zu einer ungewöhnlichen Zeit oder in ungewöhnlichem Zustand gefunden", "Gevonden op een ongebruikelijke plaats, op een ongebruikelijk moment of in een ongebruikelijke toestand"},
			"detail":     {"Found in an unusual place, at an unusual time, or in an unusual condition", "An einem ungewöhnlichen Ort, zu einer ungewöhnlichen Zeit oder in ungewöhnlichem Zustand gefunden", "Gevonden op een ongebruikelijke plaats, op een ongebruikelijk moment of in een ongebruikelijke toestand"},
			"nature":     {"Natural or human cause", "Natürliche oder menschliche Ursache", "Natuurlijke of menselijke oorzaak"},
			"indication": {"(juvenile, baby, hedgehog out during the day, birds on the ground, stuck in a natural hole, ...)", "(Jungtier, Baby, Igel tagsüber, Vögel am Boden, in einem natürlichen Loch feststeckend, ...)", "(juveniel, baby, egel overdag, vogels op de grond, vastzittend in een natuurlijk gat, ...)"},
		},
		"3.1": {
			"cause":      {"Other human activities (electrocution, mistreatment, …)", "Andere menschliche Aktivitäten (Stromschlag, Misshandlung, …)", "Andere menselijke activiteiten (elektrocutie, mishandeling, …)"},
			"detail":     {"Other human activities (electrocution, mistreatment, …)", "Andere menschliche Aktivitäten (Stromschlag, Misshandlung, …)", "Andere menselijke activiteiten (elektrocutie, mishandeling, …)"},
			"nature":     {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
			"indication": {"(electrocution, mistreatment, abandoned, pool, poisoning, kept by the finder, …)", "(Stromschlag, Misshandlung, ausgesetzt, Pool, Vergiftung, vom Finder behalten, …)", "(elektrocutie, mishandeling, achtergelaten, zwembad, vergiftiging, door de vinder gehouden, …)"},
		},
		"4.1": {
			"cause":  {"Habitat destruction/disturbance", "Zerstörung/Störung des Lebensraums", "Vernietiging/verstoring van de habitat"},
			"detail": {"Fire, blaze", "Feuer, Brand", "Brand, vuur"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"4.2": {
			"cause":  {"Habitat destruction/disturbance", "Zerstörung/Störung des Lebensraums", "Vernietiging/verstoring van de habitat"},
			"detail": {"Tree felling, …", "Baumfällung, …", "Boomkap, …"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"4.3": {
			"cause":  {"Habitat destruction/disturbance", "Zerstörung/Störung des Lebensraums", "Vernietiging/verstoring van de habitat"},
			"detail": {"Construction works", "Bauarbeiten", "Werkzaamheden"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"4.4": {
			"cause":  {"Habitat destruction/disturbance", "Zerstörung/Störung des Lebensraums", "Vernietiging/verstoring van de habitat"},
			"detail": {"Farming activity", "Landwirtschaftliche Tätigkeit", "Landbouwactiviteit"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"5.1": {
			"cause":  {"Gardening accident", "Gartenunfall", "Tuinongeval"},
			"detail": {"pitchfork, hedge trimming, ...", "Gabel, Heckenschnitt, ...", "vork, haagknippen, ..."},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"5.2": {
			"cause":  {"Gardening accident", "Gartenunfall", "Tuinongeval"},
			"detail": {"robotic mower, brush cutter", "Mähroboter, Freischneider", "maairobot, bosmaaier"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"6.1": {
			"cause":      {"Hydrocarbon pollution (oiling or oil …..)", "Kohlenwasserstoffverschmutzung (Ölung oder Öl…..)", "Koolwaterstofvervuiling (olievervuiling of olie…..)"},
			"detail":     {"Hydrocarbon pollution (oiling or oil …..)", "Kohlenwasserstoffverschmutzung (Ölung oder Öl…..)", "Koolwaterstofvervuiling (olievervuiling of olie…..)"},
			"nature":     {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
			"indication": {"(oiling or oil …..) or classify other pollution and chemical products here?", "(Ölung oder Öl…..) oder andere Verschmutzungen und Chemikalien hier einordnen?", "(olievervuiling of olie…..) of andere vervuiling en chemische producten hier plaatsen?"},
		},
		"7.1": {
			"cause":  {"Stuck - Trapped", "Feststeckend - Eingeschlossen", "Vastzittend - Opgesloten"},
			"detail": {"Chimney", "Schornstein", "Schoorsteen"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"7.2": {
			"cause":  {"Stuck - Trapped", "Feststeckend - Eingeschlossen", "Vastzittend - Opgesloten"},
			"detail": {"Fence/barbed wire", "Zaun/Stacheldraht", "Hek/prikkeldraad"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"7.3": {
			"cause":  {"Stuck - Trapped", "Feststeckend - Eingeschlossen", "Vastzittend - Opgesloten"},
			"detail": {"Other", "Andere", "Andere"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"8.1": {
			"cause":  {"Collision", "Kollision", "Botsing"},
			"detail": {"Window strike", "Kollision mit Glasfläche", "Botsing met raam"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"8.2": {
			"cause":  {"Collision", "Kollision", "Botsing"},
			"detail": {"Vehicle collision", "Kollision mit Fahrzeug", "Botsing met voertuig"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"8.3": {
			"cause":      {"Collision", "Kollision", "Botsing"},
			"detail":     {"Other collision (plane, wind turbine, train…)", "Andere Kollision (Flugzeug, Windrad, Zug…)", "Andere botsing (vliegtuig, windturbine, trein…)"},
			"nature":     {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
			"indication": {"(plane, wind turbine, train…)", "(Flugzeug, Windrad, Zug…)", "(vliegtuig, windturbine, trein…)"},
		},
		"9.1": {
			"cause":  {"Hunting, Fishing", "Jagd, Fischerei", "Jacht, Visserij"},
			"detail": {"Gunshot", "Schuss", "Schot"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"9.2": {
			"cause":  {"Hunting, Fishing", "Jagd, Fischerei", "Jacht, Visserij"},
			"detail": {"Fishhooks, fishing line", "Haken, Angelschnur", "Vishaken, vislijn"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"10.1": {
			"cause":  {"Pest control trapping", "Schädlingsbekämpfungsfalle", "Ongediertebestrijding (vallen)"},
			"detail": {"Glue trap", "Leimfalle", "Kleefval"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"10.2": {
			"cause":  {"Pest control trapping", "Schädlingsbekämpfungsfalle", "Ongediertebestrijding (vallen)"},
			"detail": {"Physical traps (fyke, cage, snare, jaw...)", "Physische Fallen (Reuse, Kastenfalle, Schlinge, Zange..)", "Fysieke vallen (fuik, klemval, strik, bek..)"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"11.1": {
			"cause":  {"Predation - Fight", "Prädation - Kampf", "Predatie - Gevecht"},
			"detail": {"Cat", "Katze", "Kat"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"11.2": {
			"cause":  {"Predation - Fight", "Prädation - Kampf", "Predatie - Gevecht"},
			"detail": {"Dog", "Hund", "Hond"},
			"nature": {"Human activity", "Menschliche Aktivität", "Menselijke activiteit"},
		},
		"11.3": {
			"cause":      {"Predation - Fight", "Prädation - Kampf", "Predatie - Gevecht"},
			"detail":     {"Wild animal", "Wildtier", "Wild dier"},
			"nature":     {"Natural cause", "Natürliche Ursache", "Natuurlijke oorzaak"},
			"indication": {"Injuries caused by an animal in the inter- or intraspecific environment", "Verletzungen durch ein Tier in der intra- oder interspezifischen Umgebung", "Verwondingen door een dier in de intra- of interspecifieke omgeving"},
		},
		"12.1": {
			"cause":      {"Weather conditions", "Wetterbedingungen", "Weersomstandigheden"},
			"detail":     {"Weather conditions", "Wetterbedingungen", "Weersomstandigheden"},
			"nature":     {"Natural cause", "Natürliche Ursache", "Natuurlijke oorzaak"},
			"indication": {"Cold, heat, wind, storm, heatwave…", "Kälte, Hitze, Wind, Sturm, Hitzewelle…", "Kou, hitte, wind, storm, hittegolf…"},
		},
		"13.1": {
			"cause":      {"Transfer", "Verlegung", "Overdracht"},
			"detail":     {"Transfer", "Verlegung", "Overdracht"},
			"nature":     {"Unidentified", "Nicht identifiziert", "Niet geïdentificeerd"},
			"indication": {"Transfer from another center or shelter", "Verlegung aus einem anderen Zentrum oder einer Auffangstation", "Overdracht van een ander centrum of opvang"},
		},
		"14.1": {
			"cause":      {"Seizure", "Beschlagnahme", "Inbeslagnname"},
			"detail":     {"Seizure", "Beschlagnahme", "Inbeslagnname"},
			"nature":     {"Natural or human cause", "Natürliche oder menschliche Ursache", "Natuurlijke of menselijke oorzaak"},
			"indication": {"Animal seized by an authority", "Von einer Behörde beschlagnahmtes Tier", "Dier in beslag genomen door een autoriteit"},
		},
	},
	"native_statuses": {
		"NS1": {
			"status":     {"Native", "Heimisch", "Inheems"},
			"indication": {"The animal must be released into its natural habitat", "Das Tier muss in seinem Lebensraum ausgewildert werden", "Het dier moet worden uitgezet in zijn natuurlijk leefgebied"},
		},
		"NS2": {
			"status":     {"Exotic", "Exotisch", "Exotisch"},
			"indication": {"The animal is transferred to a shelter, parks or wildlife centers.", "Das Tier wird in eine Auffangstation, Parks oder Wildtierzentren verlegt.", "Het dier wordt overgebracht naar een opvang, parken of wildlifecentra."},
			"precision":  {"Must be transferred to a park or shelter for individuals born in captivity and for individuals born in the wild: transfer to a center within their native range (Art. 5/1 e)", "Muss für in Gefangenschaft geborene und für in freier Wildbahn geborene Tiere in einen Park oder eine Auffangstation verlegt werden: Verlegung in ein Zentrum innerhalb ihres natürlichen Verbreitungsgebiets (Art. 5/1 e)", "Moet voor in gevangenschap geboren en voor in het wild geboren individuen worden overgebracht naar een park of opvang: overbrenging naar een centrum binnen hun natuurlijk verspreidingsgebied (Art. 5/1 e)"},
		},
		"NS3": {
			"status":     {"Exotic species of concern", "Bedenkliche exotische Art", "Zorgwekkende exotische soort"},
			"indication": {"The animal must be euthanized.", "Das Tier muss eingeschläfert werden.", "Het dier moet worden geëuthanaseerd."},
			"precision":  {"In accordance with European legislation", "Gemäß der europäischen Gesetzgebung", "In overeenstemming met de Europese wetgeving"},
		},
		"NS4": {
			"status":     {"Domestic", "Domestiziert", "Gedomesticeerd"},
			"indication": {"The animal is transferred to a shelter, parks or private individuals.", "Das Tier wird in einer Auffangstation, in Parks oder bei Privatpersonen untergebracht.", "Het dier wordt overgebracht naar een opvang, parken of particulieren."},
			"precision":  {"(Art. 5/1 e)", "(Art. 5/1 e)", "(Art. 5/1 e)"},
		},
		"NS5": {
			"status":     {"Exotic / Domestic", "Exotisch / Domestiziert", "Exotisch / Gedomesticeerd"},
			"indication": {"The animal is transferred to a shelter, parks or private individuals.", "Das Tier wird in einer Auffangstation, in Parks oder bei Privatpersonen untergebracht.", "Het dier wordt overgebracht naar een opvang, parken of particulieren."},
			"precision":  {"(Art. 5/1 e)", "(Art. 5/1 e)", "(Art. 5/1 e)"},
		},
	},
	"subside_groups": {
		"SG1": {"group": {"Birds of prey, waterfowl, waders or shorebirds", "Greifvögel, Wasservögel, Watvögel oder Limikolen", "Roofvogels, watervogels of steltlopers"}},
		"SG2": {"group": {"Other birds and bats", "Andere Vögel und Fledermäuse", "Andere vogels en vleermuizen"}},
		"SG3": {"group": {"Non-flying mammals", "Nicht fliegende Säugetiere", "Niet-vliegende zoogdieren"}},
		"SG4": {"group": {"Not fundable", "Nicht förderfähig", "Niet financierbaar"}},
	},
}

// zoneTranslations are keyed by the canonical zone name (zones have UUID IDs).
var zoneTranslations = map[string]refFieldTr{
	"Centre":              {"zone": {"Center", "Zentrum", "Centrum"}},
	"Cabinet vétérinaire": {"zone": {"Veterinary clinic", "Tierarztpraxis", "Dierenkliniek"}},
	"Soft-release":        {"zone": {"Soft-release", "Soft-release", "Soft-release"}},
	"Famille d'accueil":   {"zone": {"Foster family", "Pflegefamilie", "Pleeggezin"}},
}

// applyReferenceTranslationsTx runs applyReferenceTranslations on the global
// models.DB connection (grift convention, mirrors the create* seed tasks).
func applyReferenceTranslationsTx() error {
	return applyReferenceTranslations(models.DB)
}

// refCanonicalLoader loads recordID -> field -> canonical French base value
// for one table. Used for stale-ID recovery: an existing translation matched
// by canonical value means the record is already covered (mirrors
// actions.translationValueByCanonicalValue).
type refCanonicalLoader func(c *pop.Connection) (map[string]map[string]string, error)

var refCanonicalLoaders = map[string]refCanonicalLoader{
	"entry_causes": func(c *pop.Connection) (map[string]map[string]string, error) {
		rows := models.EntryCauses{}
		if err := c.All(&rows); err != nil {
			return nil, err
		}
		out := map[string]map[string]string{}
		for _, r := range rows {
			out[r.ID] = map[string]string{"cause": r.Cause, "detail": r.Detail, "nature": r.Nature, "indication": r.Indication}
		}
		return out, nil
	},
	"native_statuses": func(c *pop.Connection) (map[string]map[string]string, error) {
		rows := models.NativeStatuses{}
		if err := c.All(&rows); err != nil {
			return nil, err
		}
		out := map[string]map[string]string{}
		for _, r := range rows {
			precision := ""
			if r.Precision.Valid {
				precision = r.Precision.String
			}
			out[r.ID] = map[string]string{"status": r.Status, "indication": r.Indication, "precision": precision}
		}
		return out, nil
	},
	"subside_groups": func(c *pop.Connection) (map[string]map[string]string, error) {
		rows := models.SubsideGroups{}
		if err := c.All(&rows); err != nil {
			return nil, err
		}
		out := map[string]map[string]string{}
		for _, r := range rows {
			out[r.ID] = map[string]string{"group": r.Group}
		}
		return out, nil
	},
	"zones": func(c *pop.Connection) (map[string]map[string]string, error) {
		rows := []models.Zone{}
		if err := c.All(&rows); err != nil {
			return nil, err
		}
		out := map[string]map[string]string{}
		for _, r := range rows {
			out[r.ID.String()] = map[string]string{"zone": r.Zone}
		}
		return out, nil
	},
}

// applyReferenceTranslations upserts the reference translations above.
// Idempotent: skips (table, record, field, locale) rows that already exist,
// and skips inserts when a translation for the same canonical base value
// already exists under a different (stale) record ID. Wires into db:seed.
func applyReferenceTranslations(c *pop.Connection) error {
	for table, records := range referenceTranslations {
		if err := applyRefTableTranslations(c, table, records); err != nil {
			return err
		}
	}
	// Zones: UUID primary keys — match records by canonical zone name.
	zones := []models.Zone{}
	if err := c.All(&zones); err != nil {
		return err
	}
	for _, z := range zones {
		if tr, ok := zoneTranslations[strings.TrimSpace(z.Zone)]; ok {
			if err := applyRefTableTranslations(c, "zones", map[string]refFieldTr{z.ID.String(): tr}); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyRefTableTranslations applies one table's translations; records are
// keyed by record ID (zones resolve their UUID beforehand). Existing canonical
// rows and translations are loaded once per table, then missing rows are
// inserted in bounded batches to avoid per-row startup round trips.
func applyRefTableTranslations(c *pop.Connection, table string, records map[string]refFieldTr) error {
	loadCanon, ok := refCanonicalLoaders[table]
	if !ok {
		return fmt.Errorf("no canonical loader configured for table %s", table)
	}
	canon, err := loadCanon(c)
	if err != nil {
		return fmt.Errorf("failed loading canonical rows for %s: %w", table, err)
	}

	// Existing translations keyed by (recordID|field|locale) and by
	// (field|locale|canonicalBase) for stale-ID recovery.
	existsByRecord := map[string]bool{}
	existsByBase := map[string]bool{}
	var rows []models.Translation
	if err := c.Where("table_name = ? AND locale IN ('en-US','de','nl')", table).All(&rows); err != nil {
		return fmt.Errorf("failed loading existing translations for %s: %w", table, err)
	}
	for _, r := range rows {
		existsByRecord[r.RecordID+"\x00"+r.Field+"\x00"+r.Locale] = true
		base := strings.TrimSpace(canon[r.RecordID][r.Field])
		if base != "" {
			existsByBase[r.Field+"\x00"+r.Locale+"\x00"+base] = true
		}
	}

	now := time.Now()
	missing := []models.Translation{}
	for recordID, fields := range records {
		for field, values := range fields {
			base := strings.TrimSpace(canon[recordID][field])
			if base == "" {
				// No canonical value (row missing or field empty) — nothing to
				// translate. Empty canonical fields legitimately have no
				// translations (e.g. entry_causes.indication).
				continue
			}
			for i, locale := range refLocales {
				value := values[i]
				if value == "" {
					continue
				}
				if existsByRecord[recordID+"\x00"+field+"\x00"+locale] {
					continue
				}
				if existsByBase[field+"\x00"+locale+"\x00"+base] {
					continue
				}
				missing = append(missing, models.Translation{
					ID:        uuid.Must(uuid.NewV4()),
					TableName: table,
					RecordID:  recordID,
					Field:     field,
					Locale:    locale,
					Value:     value,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
		}
	}
	// Insert in bounded batches: startup seeding can otherwise issue one
	// round-trip per translation row. Existing-key checks above preserve the
	// seed's add-only and stale-ID semantics.
	for start := 0; start < len(missing); start += 100 {
		end := start + 100
		if end > len(missing) {
			end = len(missing)
		}
		args := make([]interface{}, 0, (end-start)*8)
		values := make([]string, 0, end-start)
		for _, row := range missing[start:end] {
			values = append(values, "(?,?,?,?,?,?,?,?)")
			args = append(args, row.ID.String(), row.TableName, row.RecordID, row.Field, row.Locale, row.Value, row.CreatedAt, row.UpdatedAt)
		}
		q := "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES " + strings.Join(values, ",")
		if err := c.RawQuery(q, args...).Exec(); err != nil {
			return fmt.Errorf("failed inserting translations for %s: %w", table, err)
		}
	}
	if len(missing) > 0 {
		fmt.Printf("reference translations: inserted %d rows for %s\n", len(missing), table)
	}
	return nil
}
