package models

import (
	"testing"
)

func TestFeedingGuideValidate(t *testing.T) {
	tests := []struct {
		name    string
		guide   FeedingGuide
		wantErr bool
	}{
		{
			name:    "valid guide",
			guide:   FeedingGuide{SpeciesName: "Chouette hulotte", Stage: FeedingStageBaby, Text: "Vers de farie 3x/jour"},
			wantErr: false,
		},
		{
			name:    "missing species",
			guide:   FeedingGuide{Stage: FeedingStageAdult, Text: "Text"},
			wantErr: true,
		},
		{
			name:    "missing text",
			guide:   FeedingGuide{SpeciesName: "Hérisson", Stage: FeedingStageAdult},
			wantErr: true,
		},
		{
			name:    "invalid stage",
			guide:   FeedingGuide{SpeciesName: "Hérisson", Stage: "Vieillard", Text: "Text"},
			wantErr: true,
		},
		{
			name:    "empty stage",
			guide:   FeedingGuide{SpeciesName: "Hérisson", Text: "Text"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.guide
			verrs, err := g.Validate(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && !verrs.HasAny() {
				t.Fatalf("expected validation errors, got none: %v", verrs)
			}
			if !tt.wantErr && verrs.HasAny() {
				t.Fatalf("unexpected validation errors: %v", verrs)
			}
		})
	}
}

func TestFeedingStages(t *testing.T) {
	stages := FeedingStages()
	if len(stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(stages))
	}
	for _, s := range []string{FeedingStageBaby, FeedingStageJuvenile, FeedingStageAdult, FeedingStageSick} {
		found := false
		for _, s2 := range stages {
			if s == s2 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("stage %q missing from FeedingStages()", s)
		}
	}
	for key, i18nKey := range FeedingStageI18NKeys {
		if i18nKey == "" {
			t.Fatalf("empty i18n key for stage %q", key)
		}
	}
}
