package actions

import (
	"creaves/models"
	"sync"

	"github.com/gobuffalo/pop/v6"
)

// Reference-table cache for the small, near-static lookup tables loaded by
// typehelper.go on every request (animaltypes, zones, caretypes, animalages,
// outtaketypes, traveltypes, entry_causes). Data changes only via the admin
// CRUD resources, which call the matching Invalidate*RefCache() below.
//
// In-process by design (single app container). Returned slices/pointers are
// shared — treat as read-only. Cache entries hold base (canonical French)
// rows only; translation lookups stay per-request in the tname helpers.

var refDataCache = struct {
	sync.RWMutex
	animalTypes  *models.Animaltypes
	zones        *models.Zones
	caretypes    *models.Caretypes
	animalages   *models.Animalages
	outtakeTypes *models.Outtaketypes
	traveltypes  *models.Traveltypes
	entryCauses  *models.EntryCauses
}{}

// InvalidateAnimaltypesRefCache drops the cached animal types.
func InvalidateAnimaltypesRefCache() {
	refDataCache.Lock()
	refDataCache.animalTypes = nil
	refDataCache.Unlock()
}

// InvalidateZonesRefCache drops the cached zones (zonesMap/defZone derive
// from the same cached rows, so no separate entry exists).
func InvalidateZonesRefCache() {
	refDataCache.Lock()
	refDataCache.zones = nil
	refDataCache.Unlock()
}

// InvalidateCaretypesRefCache drops the cached care types.
func InvalidateCaretypesRefCache() {
	refDataCache.Lock()
	refDataCache.caretypes = nil
	refDataCache.Unlock()
}

// InvalidateAnimalagesRefCache drops the cached animal ages.
func InvalidateAnimalagesRefCache() {
	refDataCache.Lock()
	refDataCache.animalages = nil
	refDataCache.Unlock()
}

// InvalidateOuttaketypesRefCache drops the cached outtake types.
func InvalidateOuttaketypesRefCache() {
	refDataCache.Lock()
	refDataCache.outtakeTypes = nil
	refDataCache.Unlock()
}

// InvalidateTraveltypesRefCache drops the cached travel types.
func InvalidateTraveltypesRefCache() {
	refDataCache.Lock()
	refDataCache.traveltypes = nil
	refDataCache.Unlock()
}

// InvalidateEntryCausesRefCache drops the cached entry causes.
func InvalidateEntryCausesRefCache() {
	refDataCache.Lock()
	refDataCache.entryCauses = nil
	refDataCache.Unlock()
}

// cachedRef implements single-flight load: one goroutine loads under the
// write lock; concurrent readers wait and share the result.
// isSet must report whether the cache field holds a non-nil value — callers
// cannot rely on get() != nil because a typed nil pointer stored in an
// interface{} compares non-nil.
func cachedRef(isSet func() bool, get func() interface{}, set func(v interface{}), load func() (interface{}, error)) (interface{}, error) {
	refDataCache.RLock()
	if isSet() {
		v := get()
		refDataCache.RUnlock()
		return v, nil
	}
	refDataCache.RUnlock()

	refDataCache.Lock()
	defer refDataCache.Unlock()
	if isSet() { // another goroutine won the load
		return get(), nil
	}
	v, err := load()
	if err != nil {
		return nil, err // errors are not cached; next caller retries
	}
	set(v)
	return v, nil
}

func loadAnimalTypes(tx *pop.Connection) (*models.Animaltypes, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.animalTypes != nil },
		func() interface{} { return refDataCache.animalTypes },
		func(v interface{}) { refDataCache.animalTypes = v.(*models.Animaltypes) },
		func() (interface{}, error) {
			ts := &models.Animaltypes{}
			if err := tx.Order("name asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Animaltypes), nil
}

func loadZones(tx *pop.Connection) (*models.Zones, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.zones != nil },
		func() interface{} { return refDataCache.zones },
		func(v interface{}) { refDataCache.zones = v.(*models.Zones) },
		func() (interface{}, error) {
			ts := &models.Zones{}
			if err := tx.Order("zone asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Zones), nil
}

func loadCaretypes(tx *pop.Connection) (*models.Caretypes, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.caretypes != nil },
		func() interface{} { return refDataCache.caretypes },
		func(v interface{}) { refDataCache.caretypes = v.(*models.Caretypes) },
		func() (interface{}, error) {
			ts := &models.Caretypes{}
			if err := tx.Order("name asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Caretypes), nil
}

func loadAnimalages(tx *pop.Connection) (*models.Animalages, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.animalages != nil },
		func() interface{} { return refDataCache.animalages },
		func(v interface{}) { refDataCache.animalages = v.(*models.Animalages) },
		func() (interface{}, error) {
			ts := &models.Animalages{}
			if err := tx.Order("name asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Animalages), nil
}

func loadOuttaketypes(tx *pop.Connection) (*models.Outtaketypes, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.outtakeTypes != nil },
		func() interface{} { return refDataCache.outtakeTypes },
		func(v interface{}) { refDataCache.outtakeTypes = v.(*models.Outtaketypes) },
		func() (interface{}, error) {
			ts := &models.Outtaketypes{}
			if err := tx.Order("name asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Outtaketypes), nil
}

func loadTraveltypes(tx *pop.Connection) (*models.Traveltypes, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.traveltypes != nil },
		func() interface{} { return refDataCache.traveltypes },
		func(v interface{}) { refDataCache.traveltypes = v.(*models.Traveltypes) },
		func() (interface{}, error) {
			ts := &models.Traveltypes{}
			if err := tx.Order("name asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.Traveltypes), nil
}

func loadEntryCauses(tx *pop.Connection) (*models.EntryCauses, error) {
	v, err := cachedRef(
		func() bool { return refDataCache.entryCauses != nil },
		func() interface{} { return refDataCache.entryCauses },
		func(v interface{}) { refDataCache.entryCauses = v.(*models.EntryCauses) },
		func() (interface{}, error) {
			ts := &models.EntryCauses{}
			if err := tx.Order("sort_order asc").All(ts); err != nil {
				return nil, err
			}
			return ts, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v.(*models.EntryCauses), nil
}
