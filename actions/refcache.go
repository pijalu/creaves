package actions

import (
	"creaves/models"
	"sync"
	"time"

	"github.com/gobuffalo/pop/v6"
)

// Reference-table cache for the small, near-static lookup tables loaded by
// typehelper.go on every request (animaltypes, zones, caretypes, animalages,
// outtaketypes, traveltypes, entry_causes). Data changes only via the admin
// CRUD resources, which queue the matching Invalidate*RefCache() call for
// post-commit execution (see postcommit.go).
//
// In-process by design (single app container). Returned slices/pointers are
// shared — treat as read-only. Cache entries hold base (canonical French)
// rows only; translation lookups stay per-request in the tname helpers.
//
// TTL backstop: entries expire after refCacheTTL. This bounds the residual
// stale window that no amount of invalidation can fully close: a reader that
// loaded a slice just before an invalidation keeps serving it until the next
// invalidation or expiry.

const refCacheTTL = 60 * time.Second

type refCacheSlot struct {
	value     interface{}
	expiresAt time.Time
}

var refDataCache = struct {
	sync.RWMutex
	animalTypes  refCacheSlot
	zones        refCacheSlot
	caretypes    refCacheSlot
	animalages   refCacheSlot
	outtakeTypes refCacheSlot
	traveltypes  refCacheSlot
	entryCauses  refCacheSlot
}{}

func (s *refCacheSlot) fresh() bool {
	return s.value != nil && time.Now().Before(s.expiresAt)
}

// InvalidateAnimaltypesRefCache drops the cached animal types.
func InvalidateAnimaltypesRefCache() {
	refDataCache.Lock()
	refDataCache.animalTypes = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateZonesRefCache drops the cached zones (zonesMap/defZone derive
// from the same cached rows, so no separate entry exists).
func InvalidateZonesRefCache() {
	refDataCache.Lock()
	refDataCache.zones = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateCaretypesRefCache drops the cached care types.
func InvalidateCaretypesRefCache() {
	refDataCache.Lock()
	refDataCache.caretypes = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateAnimalagesRefCache drops the cached animal ages.
func InvalidateAnimalagesRefCache() {
	refDataCache.Lock()
	refDataCache.animalages = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateOuttaketypesRefCache drops the cached outtake types.
func InvalidateOuttaketypesRefCache() {
	refDataCache.Lock()
	refDataCache.outtakeTypes = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateTraveltypesRefCache drops the cached travel types.
func InvalidateTraveltypesRefCache() {
	refDataCache.Lock()
	refDataCache.traveltypes = refCacheSlot{}
	refDataCache.Unlock()
}

// InvalidateEntryCausesRefCache drops the cached entry causes.
func InvalidateEntryCausesRefCache() {
	refDataCache.Lock()
	refDataCache.entryCauses = refCacheSlot{}
	refDataCache.Unlock()
}

// cachedRef implements single-flight load with a TTL backstop: one goroutine
// loads under the write lock; concurrent readers wait and share the result.
// A non-nil but expired slot triggers a reload exactly like a missing one.
func cachedRef(slot *refCacheSlot, load func() (interface{}, error)) (interface{}, error) {
	refDataCache.RLock()
	if slot.fresh() {
		v := slot.value
		refDataCache.RUnlock()
		return v, nil
	}
	refDataCache.RUnlock()

	refDataCache.Lock()
	defer refDataCache.Unlock()
	if slot.fresh() { // another goroutine won the load
		return slot.value, nil
	}
	v, err := load()
	if err != nil {
		return nil, err // errors are not cached; next caller retries
	}
	slot.value = v
	slot.expiresAt = time.Now().Add(refCacheTTL)
	return v, nil
}

func loadAnimalTypes(tx *pop.Connection) (*models.Animaltypes, error) {
	v, err := cachedRef(&refDataCache.animalTypes, func() (interface{}, error) {
		ts := &models.Animaltypes{}
		if err := tx.Order("name asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Animaltypes), nil
}

func loadZones(tx *pop.Connection) (*models.Zones, error) {
	v, err := cachedRef(&refDataCache.zones, func() (interface{}, error) {
		ts := &models.Zones{}
		if err := tx.Order("zone asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Zones), nil
}

func loadCaretypes(tx *pop.Connection) (*models.Caretypes, error) {
	v, err := cachedRef(&refDataCache.caretypes, func() (interface{}, error) {
		ts := &models.Caretypes{}
		if err := tx.Order("name asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Caretypes), nil
}

func loadAnimalages(tx *pop.Connection) (*models.Animalages, error) {
	v, err := cachedRef(&refDataCache.animalages, func() (interface{}, error) {
		ts := &models.Animalages{}
		if err := tx.Order("name asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Animalages), nil
}

func loadOuttaketypes(tx *pop.Connection) (*models.Outtaketypes, error) {
	v, err := cachedRef(&refDataCache.outtakeTypes, func() (interface{}, error) {
		ts := &models.Outtaketypes{}
		if err := tx.Order("name asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Outtaketypes), nil
}

func loadTraveltypes(tx *pop.Connection) (*models.Traveltypes, error) {
	v, err := cachedRef(&refDataCache.traveltypes, func() (interface{}, error) {
		ts := &models.Traveltypes{}
		if err := tx.Order("name asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.Traveltypes), nil
}

func loadEntryCauses(tx *pop.Connection) (*models.EntryCauses, error) {
	v, err := cachedRef(&refDataCache.entryCauses, func() (interface{}, error) {
		ts := &models.EntryCauses{}
		if err := tx.Order("sort_order asc").All(ts); err != nil {
			return nil, err
		}
		return ts, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*models.EntryCauses), nil
}
