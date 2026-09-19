package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// Feeding guide life stages (canonical French keys, matching the app
// convention of storing canonical French reference values).
const (
	FeedingStageBaby     = "Bébé"
	FeedingStageJuvenile = "Juvénile"
	FeedingStageAdult    = "Adulte"
	FeedingStageSick     = "Malade"
)

// FeedingStages returns the canonical life stages in display order.
func FeedingStages() []string {
	return []string{
		FeedingStageBaby,
		FeedingStageJuvenile,
		FeedingStageAdult,
		FeedingStageSick,
	}
}

// FeedingStageI18NKeys maps canonical stages to locale keys for UI labels.
var FeedingStageI18NKeys = map[string]string{
	FeedingStageBaby:     "feeding_guide.stage.baby",
	FeedingStageJuvenile: "feeding_guide.stage.juvenile",
	FeedingStageAdult:    "feeding_guide.stage.adult",
	FeedingStageSick:     "feeding_guide.stage.sick",
}

// FeedingGuide holds a diet text ("régime alimentaire") for one species at
// one life stage. Guides are keyed by the canonical species name
// (animals.species / species.creaves_species) rather than a species foreign
// key: animals.species is free text, so a name key resolves even when no
// matching species row exists.
type FeedingGuide struct {
	ID          int       `json:"id" db:"id"`
	SpeciesName string    `json:"species_name" db:"species_name"`
	Stage       string    `json:"stage" db:"stage"`
	Text        string    `json:"text" db:"text"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// FeedingGuides is not required by pop and may be deleted
type FeedingGuides []FeedingGuide

// String is not required by pop and may be deleted
func (f FeedingGuide) String() string {
	js, _ := json.Marshal(f)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
func (f *FeedingGuide) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.Validate(
		&validators.StringIsPresent{Field: f.SpeciesName, Name: "SpeciesName"},
		&validators.StringIsPresent{Field: f.Text, Name: "Text"},
	)

	// Stage must be one of the canonical life stages.
	validStage := false
	for _, s := range FeedingStages() {
		if f.Stage == s {
			validStage = true
			break
		}
	}
	if !validStage {
		verrs.Add("stage", "Stage must be one of: Bébé, Juvénile, Adulte, Malade")
	}

	return verrs, nil
}
