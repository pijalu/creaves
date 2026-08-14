package models

import (
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// SupportedLocales lists the locales accepted in the translations table.
var SupportedLocales = []string{"en-US", "fr", "de", "nl"}

// Translation holds a localized value for one field of one record.
// The base column on the source table remains the canonical French value.
type Translation struct {
	ID        uuid.UUID `json:"id" db:"id"`
	TableName string    `json:"table_name" db:"table_name"`
	RecordID  string    `json:"record_id" db:"record_id"`
	Field     string    `json:"field" db:"field"`
	Locale    string    `json:"locale" db:"locale"`
	Value     string    `json:"value" db:"value"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Translations is a list of translations.
type Translations []Translation

// Validate validates a Translation.
func (t *Translation) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: t.TableName, Name: "TableName"},
		&validators.StringIsPresent{Field: t.RecordID, Name: "RecordID"},
		&validators.StringIsPresent{Field: t.Field, Name: "Field"},
		&validators.StringIsPresent{Field: t.Value, Name: "Value"},
		&translationLocaleIsSupported{Locale: t.Locale},
	), nil
}

type translationLocaleIsSupported struct {
	Locale string
}

func (v *translationLocaleIsSupported) IsValid(errors *validate.Errors) {
	for _, l := range SupportedLocales {
		if v.Locale == l {
			return
		}
	}
	errors.Add("locale", strings.Join(SupportedLocales, ",")+" are the supported locales, got '"+v.Locale+"'")
}
