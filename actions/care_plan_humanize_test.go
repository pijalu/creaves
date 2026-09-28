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
