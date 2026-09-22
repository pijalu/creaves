package actions

import (
	"github.com/gobuffalo/buffalo/binding"
	"github.com/gofrs/uuid"
)

// registerUUIDDecoder makes form binding tolerant of empty UUID inputs.
//
// Several models (Animal, Discovery, Species, ...) carry non-nullable
// uuid.UUID fields that are bound straight from HTML form controls. A
// control with no selection (e.g. the discoverer picker's hidden
// Discovery.Discoverer.ID input, left empty when no discoverer was picked)
// submits an empty string, which the stock binder feeds to
// uuid.UnmarshalText and fails the whole request with
// "uuid: incorrect UUID length 0 in string \"\"" (HTTP 500). Treating an
// empty value as uuid.Nil lets the handlers apply their usual "no
// selection" semantics (keep existing link, skip optional FK, ...).
func registerUUIDDecoder() {
	binding.RegisterCustomDecoder(func(vals []string) (interface{}, error) {
		if len(vals) == 0 || vals[0] == "" {
			return uuid.Nil, nil
		}
		return uuid.FromString(vals[0])
	}, []interface{}{uuid.UUID{}}, nil)
}
