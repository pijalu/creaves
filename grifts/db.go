package grifts

import "github.com/gobuffalo/grift/grift"

var _ = grift.Namespace("db", func() {
	grift.Desc("seed", "Seeds a database")
	grift.Add("seed", func(c *grift.Context) error {
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
		if err := createLocality(c); err != nil {
			return err
		}
		if err := createZones(c); err != nil {
			return err
		}
		if err := createNativeStatus(c); err != nil {
			return err
		}
		return nil
	})

})
