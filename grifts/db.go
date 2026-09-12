package grifts

import (
	"creaves/actions"

	"github.com/gobuffalo/grift/grift"
)

var _ = grift.Namespace("species", func() {
	grift.Desc("repair_links", "Repairs empty species animal-type links from the approved mapping")
	grift.Add("repair_links", func(c *grift.Context) error { return repairSpeciesAnimaltypeLinks() })
})

var _ = grift.Namespace("db", func() {
	grift.Desc("seed", "Seeds a database")
	grift.Add("seed", func(c *grift.Context) error {
		if err := seedStartup(c); err != nil {
			return err
		}
		// Startup reconciliation can rename/remove reference rows; ensure any
		// request-process cache cannot serve pre-seed animal types.
		actions.InvalidateAnimaltypesRefCache()
		if err := createAdmin(c); err != nil {
			return err
		}
		if err := createAnimalage(c); err != nil {
			return err
		}
		if err := createAnimaltypes(c); err != nil {
			return err
		}
		if err := createOuttaketype(c); err != nil {
			return err
		}
		if err := createRequiredOuttaketype(c); err != nil {
			return err
		}
		if err := createCaretype(c); err != nil {
			return err
		}
		if err := extendCaretype(c); err != nil {
			return err
		}
		if err := createTraveltype(c); err != nil {
			return err
		}
		if err := createDrugs(c); err != nil {
			return err
		}
		if err := createSpecies(c); err != nil {
			return err
		}
		if err := repairSpeciesAnimaltypeLinks(); err != nil {
			return err
		}
		if err := createLocality(c); err != nil {
			return err
		}
		if err := createZones(c); err != nil {
			return err
		}
		if err := createNativeStatus(c); err != nil {
			return err
		}
		if err := createSubsideGroup(c); err != nil {
			return err
		}
		if err := createEntryCause(c); err != nil {
			return err
		}
		if err := applyReferenceTranslationsTx(); err != nil {
			return err
		}
		if err := createConfig(c); err != nil {
			return err
		}
		return nil
	})

})
