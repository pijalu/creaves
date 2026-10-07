package grifts

import (
	"database/sql"
	"errors"
	"fmt"

	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
	"github.com/gofrs/uuid"
)

// outtakeLocationOptionSeeds mirrors the #205 item-3 reference table
// (outtake_location_options.csv): 37 care centers. The 4 pre-existing
// synthetic UUIDs are kept by the ticket CSV (two renamed to proper-noun
// center names); the other 33 rows are new. Base (canonical) names stay
// French; only the generic legacy rows keep en-US/de/nl translations
// (reference_translations.go).
var outtakeLocationOptionSeeds = []struct {
	id   string
	name string
}{
	{"08d1e0ad-43ea-4d49-9bae-c40159596bdd", "CREAVES de Hotton"},
	{"0dae39b9-e7a2-4c11-8494-a0a228a11fe5", "CREAVES des Terrils"},
	{"128daff1-203b-418c-b712-5bd72c21d3f1", "Refuge Carapace"},
	{"1ad44496-dcbf-42f5-ab80-cc8d14bf24c0", "CREAVES de Namur"},
	{"1ea2b4bb-db77-4a9d-b15a-1900d80a7675", "CREAVES de Ransart"},
	{"20fbf1d2-100b-4bfa-bac4-4c4b1424148c", "CREAVES « Le Martinet »"},
	{"2d06d092-2e42-4f63-b57e-05f721800f20", "CREAVES de Sprimont"},
	{"2f813174-ca04-4669-bfc7-cf390757ec47", "CREAVES de Virelles"},
	{"3d557823-fe08-4757-8745-7a70da6b6b45", "VOC Wilde Dieren in Nood"},
	{"3fb771db-14ae-4e37-bf63-2c145fbfcf8c", "VOC Egel Hulpcentrum Avalon"},
	{"5813dda7-ce51-46f8-8b9f-df9b6689028a", "CREAVES de St Hubert"},
	{"5c32e283-34fc-4fed-9a1b-d47861f4efcb", "CREAVES de BIrds Bay"},
	{"5ec4bede-d886-4fff-a0d5-6d8b8c06e54b", "Centre de Soins Bruxelles - LRBPO"},
	{"64696459-79f6-4c2e-bbb9-0a5c7ea568a6", "VOC Malderen"},
	{"78a9ef05-84dd-4934-9436-c21decf68df7", "CREAVES « l’hermitage à Thimister »"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01", "CREAVES de Pairai Daiza"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b02", "Refuge"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03", "VOC Oostende"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b04", "Zoo"},
	{"858aec3e-a02e-4cd5-a325-1610d61e1508", "refuge Opale"},
	{"99d36fe4-6ab0-45dd-b0ee-fe0e07baa6c1", "CREAVES « le Chalet des Hirchons »"},
	{"ad8c55f7-2f27-4b3b-90a5-7cb5e449e23e", "VOC Natuurhulpcentrum"},
	{"b5cd01ad-48f5-486b-ab19-4e4b2c67e6a4", "CREAVES de Dour"},
	{"bb096932-1d04-41ec-8351-2598e5c76a6f", "SOS Wilde Dieren"},
	{"bd573bf6-8beb-465f-b517-40e5b6091122", "VOC Merelbeke"},
	{"bf9ed094-9a0f-4834-9655-30df16bc65f3", "Refuge L'Arche"},
	{"c87b56f2-a1e6-4c20-bfd4-a27796047a5d", "Zoo Pairi-Daiza"},
	{"c91fd21d-c64a-4937-a3c1-051c7144c313", "CREAVES de Murringen"},
	{"ca08b33f-44c1-4e59-9b77-11e64756416e", "CREAVES  « La Tanière des Fagnes »"},
	{"d062f8c9-7429-4bb1-bc11-0378829b9b51", "VOC Beernem"},
	{"d1349eae-80e4-41a7-a928-389fa84eea5e", "CREAVES de Frasnes-lez-Anvaing"},
	{"d40853ac-44b1-4f78-a31d-6eac4f407122", "VOC Neteland"},
	{"d558f8ae-c6a7-4e14-a140-d001381fea05", "CREAVES d’Herbeumont"},
	{"dfd276e2-36f5-4939-94eb-fd2956503a21", "Zoo Domaine des Grottes de Han"},
	{"e1866cb3-0562-4b87-8bb6-3a3af0f71021", "CREAVES L'Arche de Lorraine"},
	{"e224d609-71c4-41e7-b26b-5c5710c9d40b", "CREAVES de Perwez"},
	{"ec5043c8-93ec-43ba-acbb-325a40ba24f8", "CREAVES d'Andenne"},
}

// createOuttakeLocationOptions seeds the outtake_location_options reference
// list (used by outcome types with location_mode = "list"). Rows are matched
// by ID (stable UUIDs): missing rows are inserted, drifted canonical names are
// force-corrected. Idempotent.
func createOuttakeLocationOptions(c *grift.Context) error {
	inserted, corrected := 0, 0
	for _, s := range outtakeLocationOptionSeeds {
		id := uuid.Must(uuid.FromString(s.id))
		row := &models.OuttakeLocationOption{}
		err := models.DB.Find(row, id)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			fmt.Printf("Creating outtake location option %s\n", s.name)
			if err := models.DB.Create(&models.OuttakeLocationOption{
				ID:   id,
				Name: s.name,
			}); err != nil {
				return err
			}
			inserted++
			continue
		}
		if row.Name != s.name {
			fmt.Printf("Correcting outtake location option %s -> %s\n", row.Name, s.name)
			row.Name = s.name
			if err := models.DB.Update(row); err != nil {
				return err
			}
			corrected++
		}
	}
	fmt.Printf("outtake location options: %d inserted, %d name corrections, %d already up to date\n",
		inserted, corrected, len(outtakeLocationOptionSeeds)-inserted-corrected)
	return nil
}
