package actions

import (
	"testing"

	"creaves/locales"

	i18n "github.com/gobuffalo/mw-i18n/v2"
	"github.com/stretchr/testify/require"
)

// humanizerTestTranslate loads the real locale files once and returns a tr
// func for one language, so the humanizer tests assert against the shipped
// translations (missing keys fail loudly instead of passing on stubs).
func humanizerTestTranslate(t *testing.T, lang string) func(string, map[string]interface{}) string {
	t.Helper()
	tr, err := i18n.New(locales.FS(), "en-US")
	require.NoError(t, err)
	return func(id string, args map[string]interface{}) string {
		var s string
		var err error
		if args == nil {
			s, err = tr.TranslateWithLang(lang, id)
		} else {
			s, err = tr.TranslateWithLang(lang, id, args)
		}
		require.NoError(t, err, "translation %q (%s)", id, lang)
		return s
	}
}

// TestHumanizeSchedule (bugs.md U12): the §4.3 JSON document renders as a
// human sentence in every supported locale — never as raw JSON.
func TestHumanizeSchedule(t *testing.T) {
	raw := `{"times": ["12:00"], "anchor": "fixed", "every_days": 1, "anchor_date": "2026-09-29", "duration_days": 1}`
	cases := []struct{ lang, want string }{
		{"en-US", "Every day at 12:00, from 2026-09-29, for 1 day(s)"},
		{"fr", "Chaque jour à 12:00, à partir du 2026-09-29, pendant 1 jour(s)"},
		{"de", "Jeden Tag um 12:00, ab 2026-09-29, für 1 Tag(e)"},
		{"nl", "Elke dag om 12:00, vanaf 2026-09-29, gedurende 1 dag(en)"},
	}
	for _, c := range cases {
		got := humanizeScheduleWith(raw, humanizerTestTranslate(t, c.lang))
		require.Equal(t, c.want, got, "lang %s", c.lang)
	}
}

func TestHumanizeScheduleVariants(t *testing.T) {
	tr := humanizerTestTranslate(t, "en-US")
	require.Equal(t,
		"Every 2 days at 08:00, 18:00, from intake, open-ended",
		humanizeScheduleWith(`{"times":["08:00","18:00"],"every_days":2}`, tr))
	require.Equal(t,
		"Every day at 09:00, on Mon, Wed, from intake, for 5 day(s)",
		humanizeScheduleWith(`{"times":["09:00"],"weekdays":[1,3],"duration_days":5}`, tr))
	// Unparseable → verbatim fallback (display-only path).
	require.Equal(t, `{"broken`, humanizeScheduleWith(`{"broken`, tr))
}

// TestHumanizeScheduleRepeatWindow (e2e/bugs.md BUG-51): a repeat-mode
// schedule renders as the COMPACT window form in every locale — never the
// (potentially huge) expanded slot list. The meal count rides along; the
// overnight window (end < start) stays verbatim; a multi-day cadence keeps
// its "every N days" prefix.
func TestHumanizeScheduleRepeatWindow(t *testing.T) {
	raw := `{"times":["09:00","13:00","17:00"],"every_days":1,"anchor":"fixed","anchor_date":"2026-10-14","duration_days":5,"repeat":{"from":"09:00","to":"17:00","every_minutes":240}}`
	cases := []struct{ lang, want string }{
		{"en-US", "Every day from 09:00 to 17:00 every 04:00 h (3 meals), from 2026-10-14, for 5 day(s)"},
		{"fr", "Chaque jour de 09:00 à 17:00 toutes les 04:00 h (3 repas), à partir du 2026-10-14, pendant 5 jour(s)"},
		{"de", "Jeden Tag von 09:00 bis 17:00 alle 04:00 h (3 Mahlzeiten), ab 2026-10-14, für 5 Tag(e)"},
		{"nl", "Elke dag van 09:00 tot 17:00 elke 04:00 u (3 maaltijden), vanaf 2026-10-14, gedurende 5 dag(en)"},
	}
	for _, c := range cases {
		got := humanizeScheduleWith(raw, humanizerTestTranslate(t, c.lang))
		require.Equal(t, c.want, got, "lang %s", c.lang)
	}

	tr := humanizerTestTranslate(t, "fr")
	// Overnight window: end < start verbatim (the editor shows from=22:00 to=06:00);
	// times are sorted per the §4.3 invariant (ExpandRepeat order).
	require.Contains(t,
		humanizeScheduleWith(`{"times":["02:00","06:00","22:00"],"every_days":1,"repeat":{"from":"22:00","to":"06:00","every_minutes":240}}`, tr),
		"de 22:00 à 06:00 toutes les 04:00 h (3 repas)")
	// Multi-day cadence keeps its prefix.
	require.Contains(t,
		humanizeScheduleWith(`{"times":["08:00","18:00"],"every_days":2,"repeat":{"from":"08:00","to":"18:00","every_minutes":600}}`, tr),
		"Tous les 2 jours de 08:00 à 18:00 toutes les 10:00 h (2 repas)")
	// Slot-mode (no repeat block) stays the plain slot list (regression).
	require.Contains(t,
		humanizeScheduleWith(`{"times":["08:00","18:00"],"every_days":1}`, tr),
		"Chaque jour à 08:00, 18:00")
}

// TestRichPlanName (bugs.md U15): a content-free converted name gains the
// payload content; a meaningful name is untouched (no duplication).
func TestRichPlanName(t *testing.T) {
	cases := []struct {
		name, kind, payload, want string
	}{
		{"Alimentation — nb 1/2 (conversion)", "feeding", `{"food":"Nutribird A21","caretype_id":"x"}`,
			"Alimentation — nb 1/2 — Nutribird A21"},
		{"Gavage — Nutribird A21", "feeding", `{"food":"Nutribird A21"}`,
			"Gavage — Nutribird A21"}, // content already in name → no dup
		{"Traitement (conversion)", "medication", `{"drug":"amoxiclav","dosage":"0.05 ml"}`,
			"Traitement — amoxiclav — 0.05 ml"},
		{"", "medication", `{"drug":"amoxiclav","dosage":"0.05 ml"}`,
			"amoxiclav — 0.05 ml"},
		{"Observation quotidienne", "observation", `{"prompt":"Mange seul ?"}`,
			"Observation quotidienne — Mange seul ?"},
		{"Soin de plaie", "care", `{"instructions":"nettoyer 2x/j"}`,
			"Soin de plaie — nettoyer 2x/j"}, // content appended when absent from name
		{"Plain name", "care", `{"note":"nettoyer"}`, "Plain name — nettoyer"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, richPlanName(c.name, c.kind, []byte(c.payload)), "%+v", c)
	}
}
