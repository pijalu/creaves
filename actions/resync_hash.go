package actions

import (
	crypto_sha256 "crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"creaves/models"
	"github.com/gofrs/uuid"
)

var NamespaceCreavesState = uuid.Must(uuid.FromString("9f6d3e20-7c1a-4b8f-9e2a-3d4c5b6a7f10"))

func StateContentHash(fields ...string) string {
	h := crypto_sha256.Sum256([]byte(strings.Join(fields, "\x1f")))
	return hex.EncodeToString(h[:])
}

// CanonicalStateContent returns the content-addressed representation specified
// for animal_state events. Volatile audit fields (timestamp and user fields)
// are deliberately not included.
func CanonicalStateContent(instanceID string, payload models.EventPayload) string {
	fields := []string{
		instanceID, fmt.Sprintf("%d", payload.Animal.ID),
		fmt.Sprintf("%d", payload.Animal.Year), fmt.Sprintf("%d", payload.Animal.YearNumber),
		payload.Animal.Species, payload.Animal.Gender, payload.Animal.Cage,
		payload.Animal.Zone, payload.Animal.Ring, payload.Animal.AnimalType,
		payload.Animal.AnimalAge, payload.CurrentStatus,
		payload.Discovery.Location, payload.Discovery.PostalCode, payload.Discovery.City,
		payload.Discovery.Date, payload.Discovery.EntryCause, payload.Discovery.Reason,
		payload.Intake.Date, payload.Intake.General, payload.Intake.Wounds,
		payload.Intake.Parasites, payload.Intake.Remarks,
		payload.Outtake.Date, payload.Outtake.Type, payload.Outtake.Location,
		SortedTranslationsJSON(payload.Translations),
	}
	return strings.Join(fields, "\x1f")
}

// StateContentHashPayload computes SHA-256 over CanonicalStateContent.
func StateContentHashPayload(instanceID string, payload models.EventPayload) string {
	return StateContentHash(CanonicalStateContent(instanceID, payload))
}

func StateEventUUID(instanceID string, animalID int, contentHash string) uuid.UUID {
	return uuid.NewV5(NamespaceCreavesState, fmt.Sprintf("%s|%d|%s", instanceID, animalID, contentHash))
}

func SortedTranslationsJSON(translations map[string]map[string]string) string {
	locales := make([]string, 0, len(translations))
	for locale := range translations {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	var b strings.Builder
	b.WriteByte('{')
	for i, locale := range locales {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%q:", locale))
		fields := make([]string, 0, len(translations[locale]))
		for field := range translations[locale] {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		b.WriteByte('{')
		for j, field := range fields {
			if j > 0 {
				b.WriteByte(',')
			}
			value, _ := json.Marshal(translations[locale][field])
			b.WriteString(fmt.Sprintf("%q:%s", field, value))
		}
		b.WriteByte('}')
	}
	b.WriteByte('}')
	return b.String()
}
