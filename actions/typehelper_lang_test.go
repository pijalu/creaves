package actions

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCurrentLangUsesResolvedLanguages guards the bug where the UI rendered
// in French (Accept-Language resolved by the i18n middleware, no lang cookie)
// while DB-backed names (species etc.) were translated to English because
// currentLang only read the cookie and defaulted to en-US.
func TestCurrentLangUsesResolvedLanguages(t *testing.T) {
	cases := []struct {
		name      string
		languages []string // nil = not set in context
		cookie    string   // "" = no cookie
		want      string
	}{
		// i18n-resolved languages win: FR UI must yield canonical French names.
		{"accept-language fr, no cookie", []string{"fr", "en-US"}, "", ""},
		{"accept-language fr-FR folded", []string{"fr-FR", "fr", "en-US"}, "", ""},
		{"accept-language de regional fold", []string{"de-AT", "de"}, "", "de"},
		{"accept-language nl", []string{"nl"}, "", "nl"},
		{"default en-US (i18n always appends default)", []string{"en-US"}, "", "en-US"},
		{"cookie fr resolved as languages[0]", []string{"fr", "en-US"}, "fr", ""},
		{"cookie en-US resolved as languages[0]", []string{"en-US"}, "en-US", "en-US"},
		// Fallback paths when the i18n middleware did not run.
		{"no languages, cookie fr", nil, "fr", ""},
		{"no languages, cookie en-US", nil, "en-US", "en-US"},
		{"no languages, cookie de", nil, "de", "de"},
		{"no languages, no cookie -> i18n default", nil, "", "en-US"},
		// Unsupported language: UI falls back to base English templates, so
		// DB names follow en-US as well.
		{"unsupported es falls back to en-US", []string{"es", "en-US"}, "", "en-US"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "lang", Value: tc.cookie})
			}
			c := newRALContext(req)
			if tc.languages != nil {
				c.Set("languages", tc.languages)
			}
			if got := currentLang(c); got != tc.want {
				t.Errorf("currentLang() = %q, want %q", got, tc.want)
			}
		})
	}
}
