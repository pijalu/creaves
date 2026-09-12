package models

import "testing"

func TestMaskedWebhookAPIKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want string
	}{
		{"empty", "", ""},
		{"short", "abc", "••••"},
		{"exactly4", "abcd", "••••"},
		{"typical", "creaves_ab12cd34", "••••cd34"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := ConfigSettings{WebhookAPIKey: tc.key}
			if got := s.MaskedWebhookAPIKey(); got != tc.want {
				t.Fatalf("MaskedWebhookAPIKey() = %q, want %q", got, tc.want)
			}
			// The full secret must never appear in the masked output
			// (unless the key is 4 chars or less, in which case only
			// dots are shown).
			if tc.key != "" && len(tc.key) > 4 && s.MaskedWebhookAPIKey() == tc.key {
				t.Fatalf("masked value leaked the full key")
			}
		})
	}
}
