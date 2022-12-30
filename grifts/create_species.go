
package grifts

import (
	"creaves/models"
	"fmt"
	"strconv"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

func createSpecies(c *Context) error {
	ts := []struct {
		Species        string        
		Group          string        
		Family         string       
		CreavesSpecies string        
		CreavesGroup   string        
		Subside        string
	}{

{
	Species        : "Corvus corax",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Grand Corbeau",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Capreolus capreolus",
	Group          : "Mammifères", 
	Family         : "Cervidés",
	CreavesSpecies : "Chevreuil",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Cervus elaphus",
	Group          : "Mammifères", 
	Family         : "Cervidés",
	CreavesSpecies : "Cerf élaphe",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "0",
},

{
	Species        : "Sus scrofa",
	Group          : "Mammifères", 
	Family         : "Suidés",
	CreavesSpecies : "Sanglier",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "0",
},

{
	Species        : "Canis lupus",
	Group          : "Mammifères", 
	Family         : "Canidés",
	CreavesSpecies : "Loup",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Felis silvestris",
	Group          : "Mammifères", 
	Family         : "Félidés",
	CreavesSpecies : "Chat sylvestre",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "4.51",
},

{
	Species        : "Lutra lutra",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Loutre",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Martes foina",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Fouine",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Martes martes",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Martre",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Meles meles",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Blaireau",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "4.51",
},

{
	Species        : "Mustela erminea",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Hermine",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Mustela nivalis",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Belette",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Mustela putorius",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Putois",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Vulpes vulpes",
	Group          : "Mammifères", 
	Family         : "Canidés",
	CreavesSpecies : "Renard roux",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Barbastella barbastellus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Barbastelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Eptesicus nilssonii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Eptesicus serotinus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Sérotine",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis alcathoe",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis bechsteinii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis brandtii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis dasycneme",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis daubentonii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis emarginatus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis myotis",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis mystacinus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Myotis nattereri",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Murin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Nyctalus leisleri",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Noctule",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Nyctalus noctula",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Noctule",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Pipistrellus kuhlii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Pipistrelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Pipistrellus nathusii",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Pipistrelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Pipistrellus pipistrellus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Pipistrelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Pipistrellus pygmaeus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Pipistrelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Plecotus auritus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Oreillard",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Plecotus austriacus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Oreillard",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Rhinolophus ferrumequinum",
	Group          : "Mammifères", 
	Family         : "Rhinolophidés",
	CreavesSpecies : "Rhinolophe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Rhinolophus hipposideros",
	Group          : "Mammifères", 
	Family         : "Rhinolophidés",
	CreavesSpecies : "Rhinolophe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Vespertilio murinus",
	Group          : "Mammifères", 
	Family         : "Verspertilionidés",
	CreavesSpecies : "Serotine",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.75",
},

{
	Species        : "Crocidura leucodon",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Crocidura russula",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Erinaceus europaeus",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Hérisson",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "1.5",
},

{
	Species        : "Neomys anomalus",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Neomys fodiens",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Sorex araneus",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Sorex coronatus",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Sorex minutus",
	Group          : "Mammifères", 
	Family         : "Soricidés",
	CreavesSpecies : "Musaraigne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Talpa europaea",
	Group          : "Mammifères", 
	Family         : "Talpidés",
	CreavesSpecies : "Taupe",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Lepus europaeus",
	Group          : "Mammifères", 
	Family         : "Léporidés",
	CreavesSpecies : "Lièvre",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Oryctolagus cuniculus",
	Group          : "Mammifères", 
	Family         : "Léporidés",
	CreavesSpecies : "Lapin de Garenne",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Apodemus flavicollis",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Mulot",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Apodemus sylvaticus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Mulot",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Arvicola terrestris",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Campagnol",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Clethrionomys glareolus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Campagnol",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Cricetus cricetus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Grand Hasmter",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Eliomys quercinus",
	Group          : "Mammifères", 
	Family         : "Muscardinidés",
	CreavesSpecies : "Lérot",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Glis glis",
	Group          : "Mammifères", 
	Family         : "Muscardinidés",
	CreavesSpecies : "Loir",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Micromys minutus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Rat des moissons",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Microtus agrestis",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Campagnol",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Microtus arvalis",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Campagnol",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Microtus subterraneus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Campagnol",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Mus domesticus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Souris",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Muscardinus avellanarius",
	Group          : "Mammifères", 
	Family         : "Muscardinidés",
	CreavesSpecies : "Muscardin",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Rattus rattus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Rat noir",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Sciurus vulgaris",
	Group          : "Mammifères", 
	Family         : "Sciuridés",
	CreavesSpecies : "Ecureuil roux",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "1.12",
},

{
	Species        : "Columba oenas",
	Group          : "Oiseaux", 
	Family         : "Colombidés",
	CreavesSpecies : "Pigeon colombin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Columba palumbus",
	Group          : "Oiseaux", 
	Family         : "Colombidés",
	CreavesSpecies : "Pigeon ramier",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Pterocles orientalis",
	Group          : "Oiseaux", 
	Family         : "Ptéroclicidés",
	CreavesSpecies : "Ganga unibande",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Streptopelia decaocto",
	Group          : "Oiseaux", 
	Family         : "Colombidés",
	CreavesSpecies : "Tourterelle turque",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Streptopelia turtur",
	Group          : "Oiseaux", 
	Family         : "Colombidés",
	CreavesSpecies : "Tourterelle des bois",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Syrrhaptes paradoxus",
	Group          : "Oiseaux", 
	Family         : "Ptéroclicidés",
	CreavesSpecies : "Syrrhapte paradoxal",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Alcedo atthis",
	Group          : "Oiseaux", 
	Family         : "Alcédinidés",
	CreavesSpecies : "Martin-pêcheur d'Europe",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Coracias garrulus",
	Group          : "Oiseaux", 
	Family         : "Coraciidés",
	CreavesSpecies : "Rollier d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Merops apiaster",
	Group          : "Oiseaux", 
	Family         : "Méropidés",
	CreavesSpecies : "Guêpier d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Upupa epops",
	Group          : "Oiseaux", 
	Family         : "Upudidés",
	CreavesSpecies : "Huppe fasciée",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Clamator glandarius",
	Group          : "Oiseaux", 
	Family         : "Cuculidés",
	CreavesSpecies : "Coucou geai",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Coccyzus americanus",
	Group          : "Oiseaux", 
	Family         : "Cuculidés",
	CreavesSpecies : "Coulicou à bec jaune",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Cuculus canorus",
	Group          : "Oiseaux", 
	Family         : "Cuculidés",
	CreavesSpecies : "Coucou gris",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Ardea alba",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Grande Aigrette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ardea cinerea",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Héron cendré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ardea purpurea",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Héron pourpré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ardeola ralloides",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Crabier chevelu",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Botaurus stellaris",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Butor étoilé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Bubulcus ibis",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Héron garde-boeufs",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ciconia ciconia",
	Group          : "Oiseaux", 
	Family         : "Ciconiidés",
	CreavesSpecies : "Cigogne blanche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ciconia nigra",
	Group          : "Oiseaux", 
	Family         : "Ciconiidés",
	CreavesSpecies : "Cigogne noire",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Egretta garzetta",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Aigrette garzette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Grus grus",
	Group          : "Oiseaux", 
	Family         : "Gruidés",
	CreavesSpecies : "Grue cendrée",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Ixobrychus minutus",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Blongios nain",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Nycticorax nycticorax",
	Group          : "Oiseaux", 
	Family         : "Ardeidés",
	CreavesSpecies : "Bihoreau gris",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Platalea leucorodia",
	Group          : "Oiseaux", 
	Family         : "Threskiornithidés",
	CreavesSpecies : "Spatule blanche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Plegadis falcinellus",
	Group          : "Oiseaux", 
	Family         : "Threskiornithidés",
	CreavesSpecies : "Ibis falcinelle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "4.09",
},

{
	Species        : "Bonasa bonasia",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Gélinotte des bois",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Coturnix coturnix",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Caille des blés",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Perdix perdix",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Perdrix grise",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Tetrao tetrix",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Tetras lyre",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Tetrao urogallus",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Grand Tétras",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Actitis hypoleucos",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier guignette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Arenaria interpres",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Tournepierre à collier",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Burhinus oedicnemus",
	Group          : "Oiseaux", 
	Family         : "Burhinidés",
	CreavesSpecies : "Oedicnème criard",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris acuminata",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau à queue pointue",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris alba",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau sanderling",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris alpina",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau variable",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris canutus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau maubèche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris ferruginea",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau cocorli",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris acuminata",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau de Bonaparte",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris melanotos",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau tacheté",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris minuta",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau minute",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Calidris temminckii",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau de Temminck",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Charadrius alexandrinus",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Pluvier gravelot à collier interrompu",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Charadrius dubius",
	Group          : "Oiseaux", 
	Family         : "charadriidés",
	CreavesSpecies : "Pluvier petit gravelot",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Charadrius hiaticula",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Pluvier grand gravelot",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Charadrius morinellus",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Pluvier guignard",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Gallinago gallinago",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécassine des marais",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Gallinago media",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécassine double",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Glareola pratincola",
	Group          : "Oiseaux", 
	Family         : "Glaréolidés",
	CreavesSpecies : "Glaréole à collier",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Haematopus ostralegus",
	Group          : "Oiseaux", 
	Family         : "Haematopodidés",
	CreavesSpecies : "Huîtrier pie",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Himantopus himantopus",
	Group          : "Oiseaux", 
	Family         : "Récurvirostridés",
	CreavesSpecies : "Echasse blanche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Limicola falcinellus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasseau falcinelle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Limosa lapponica",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Barge rousse",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Limosa limosa",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Barge à queue noire",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Lymnocryptes minimus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécassine sourde",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Numenius arquata",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Courlis cendré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Numenius phaeopus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Courlis corlieu",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Numenius tenuirostris",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Courlis à bec grèle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Phalaropus fulicarius",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Phalarope à bec large",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Phalaropus lobatus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Phalarope à bec étroit",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Philomachus pugnax",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Combattant varié",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Pluvialis apricaria",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Pluvier doré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Pluvialis squatarola",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Pluvier argenté",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Recurvirostra avosetta",
	Group          : "Oiseaux", 
	Family         : "Récurvirostridés",
	CreavesSpecies : "Avocette élégante",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Scolopax rusticola",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Bécasse des bois",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa cinerea",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier bargette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa erythropus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier arlequin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa glareola",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier sylvain",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa nebularia",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier aboyeur",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa ochropus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier culblanc",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa stagnatilis",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier stagnatile",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tringa totanus",
	Group          : "Oiseaux", 
	Family         : "Scolopacidés",
	CreavesSpecies : "Chevalier gambette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Vanellus vanellus",
	Group          : "Oiseaux", 
	Family         : "Charadriidés",
	CreavesSpecies : "Vanneau huppé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Chlamydotis undulata",
	Group          : "Oiseaux", 
	Family         : "Otididés",
	CreavesSpecies : "Outarde houbara",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Otis tarda",
	Group          : "Oiseaux", 
	Family         : "Otididés",
	CreavesSpecies : "Outarde barbue",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Tetrax tetrax",
	Group          : "Oiseaux", 
	Family         : "Otididés",
	CreavesSpecies : "Outarde canepetière",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Alca torda",
	Group          : "Oiseaux", 
	Family         : "Alcidés",
	CreavesSpecies : "Pingouin torda",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Alle alle",
	Group          : "Oiseaux", 
	Family         : "Alcidés",
	CreavesSpecies : "Mergule nain",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas acuta",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard pilet",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas clypeata",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard souchet",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas crecca",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Sarcelle d'hiver",
	CreavesGroup   : "rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas discors",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Sarcelle à ailes bleues",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas penelope",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard siffleur",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas platyrhynchos",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard colvert",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anas strepera",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard chipeau",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anser albifrons",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Oie rieuse",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anser brachyrhynchus",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Oie à bec court",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Anser fabalis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Oie des moissons",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Aythya collaris",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Fuligule à bec cerclé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Aythya ferina",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Fuligule milouin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Aythya fuligula",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Fuligule morillon",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Aythya marila",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Fuligule milouinan",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Aythya nyroca",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Fuligule nyroca",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Branta bernicla",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Bernache cravant",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Branta leucopsis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Bernache nonnette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Branta ruficollis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Bernache à cou roux",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Bucephala clangula",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Garrot à oeil d'or",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Chlidonias hybridus",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Guifette moustac",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Chlidonias leucopterus",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Guifette leucoptère",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Chlidonias niger",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Guifette noire",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Clangula hyemalis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Harelde boréale",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Crex crex",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Râle des genêts",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Cygnus columbianus bewickii",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Cygne de Bewick",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Cygnus cygnus",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Cygne chanteur",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Fratercula arctica",
	Group          : "Oiseaux", 
	Family         : "Alcidés",
	CreavesSpecies : "Macareux moine",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Fulica atra",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Foulque macroule",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Fulmarus glacialis",
	Group          : "Oiseaux", 
	Family         : "Procellaridés",
	CreavesSpecies : "Fulmar boréal",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Gallinula chloropus",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Gallinule poule d'eau",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Gavia arctica",
	Group          : "Oiseaux", 
	Family         : "Gaviidés",
	CreavesSpecies : "Plongeon arctique",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Gavia stellata",
	Group          : "Oiseaux", 
	Family         : "Gaviidés",
	CreavesSpecies : "Plongeon catmarin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Gelochelidon nilotica",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne hansel",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Hydrobates pelagicus",
	Group          : "Oiseaux", 
	Family         : "Hydrobatidés",
	CreavesSpecies : "Océanite tempête",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus argentatus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland argenté",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus cachinnans",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland pontique",
	CreavesGroup   : "rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus canus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland cendré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus fuscus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland brun",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus glaucoides",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland à ailes blanches",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus hyperboreus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland bourgmestre",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus marinus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland marin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus melanocephalus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Mouette mélanocéphale",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus minutus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Mouette pygmée",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus ridibundus",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Mouette rieuse",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus sabini",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Mouette de Sabine",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Melanitta fusca",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Macreuse brune",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Melanitta nigra",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Macreuse noire",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Mergus albellus",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Harle piette",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Mergus merganser",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Harle bièvre",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Mergus serrator",
	Group          : "Oiseaux", 
	Family         : "Alcidés",
	CreavesSpecies : "Harle huppé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Morus bassanus",
	Group          : "Oiseaux", 
	Family         : "Sulidés",
	CreavesSpecies : "Fou de Bassan",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Netta rufina",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Nette rousse",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Oceanodroma leucorhoa",
	Group          : "Oiseaux", 
	Family         : "Hydrobatidés",
	CreavesSpecies : "Océanite culblanc",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Oxyura leucocephala",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Erismature à tête blanche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Phalacrocorax aristotelis",
	Group          : "Oiseaux", 
	Family         : "Phalacrocoracidés",
	CreavesSpecies : "Cormoran huppé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Phalacrocorax carbo",
	Group          : "Oiseaux", 
	Family         : "Phalacrocoracidés",
	CreavesSpecies : "Grand Cormoran",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Podiceps auritus",
	Group          : "Oiseaux", 
	Family         : "Podicipedidés",
	CreavesSpecies : "Grèbe esclavon",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Podiceps cristatus",
	Group          : "Oiseaux", 
	Family         : "Podicipedidés",
	CreavesSpecies : "Grèbe huppé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Podiceps grisegena",
	Group          : "Oiseaux", 
	Family         : "Podicipedidés",
	CreavesSpecies : "Grèbe jougris",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Podiceps nigricollis",
	Group          : "Oiseaux", 
	Family         : "Podicipedidés",
	CreavesSpecies : "Grèbe à cou noir",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Porzana parva",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Marouette poussin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Porzana porzana",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Marouette ponctuée",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Porzana pusilla",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Marouette de Baillon",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Rallus aquaticus",
	Group          : "Oiseaux", 
	Family         : "Rallidés",
	CreavesSpecies : "Râle d'eau",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "2.08",
},

{
	Species        : "Rissa triactyla",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Mouette tridactyle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Somateria mollissima",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Eider à duvet",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Stercorarius longicaudus",
	Group          : "Oiseaux", 
	Family         : "Stercorariidés",
	CreavesSpecies : "Labbe à longue queue",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Stercorarius parasiticus",
	Group          : "Oiseaux", 
	Family         : "Stercorariidés",
	CreavesSpecies : "Labbe parasite",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Stercorarius pomarinus",
	Group          : "Oiseaux", 
	Family         : "Stercorariidés",
	CreavesSpecies : "Labbe pomarin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Stercorarius skua",
	Group          : "Oiseaux", 
	Family         : "Stercorariidés",
	CreavesSpecies : "Grand Labbe",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Sterna albifrons",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne naine",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Sterna caspia",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne caspienne",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Sterna hirundo",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne pierregarin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Sterna paradisaea",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne arctique",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Sterna sandvicensis",
	Group          : "Oiseaux", 
	Family         : "Sternidés",
	CreavesSpecies : "Sterne caugek",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Tachybaptus ruficollis",
	Group          : "Oiseaux", 
	Family         : "Podicipedidés",
	CreavesSpecies : "Grèbe castagneux",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Tadorna tadorna",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Tadorne de Belon",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Uria aalge",
	Group          : "Oiseaux", 
	Family         : "Alcidés",
	CreavesSpecies : "Guillemot de Troïl",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Acrocephalus agricola",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Rousserolle isabelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Acrocephalus arundinaceus",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Rousserolle turdoïde",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Acrocephalus paludicola",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Phragmite aquatique",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Acrocephalus palustris",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Rousserolle verderolle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Acrocephalus schoenobaenus",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Phragmite des joncs",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Acrocephalus scirpaceus",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Rousserolle effarvatte",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Aegithalos caudatus",
	Group          : "Oiseaux", 
	Family         : "Aegithalidés",
	CreavesSpecies : "Orite à longue queue",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Alauda arvensis",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette des champs",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Anthus campestris",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit rousseline",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus cervinus",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit à gorge rousse",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus petrosus",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit maritime",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus pratensis",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit farlouse",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus richardi",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit de Richard",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus spinoletta",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit spioncelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Anthus trivialis",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Pipit des arbres",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Apus apus",
	Group          : "Oiseaux", 
	Family         : "Apodidés",
	CreavesSpecies : "Martinet noir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Apus melba",
	Group          : "Oiseaux", 
	Family         : "Apodidés",
	CreavesSpecies : "Martinet à ventre blanc",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Bombycilla garrulus",
	Group          : "Oiseaux", 
	Family         : "Bombycillidés",
	CreavesSpecies : "Jaseur boréal",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Calandrella brachydactyla",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette calandrelle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Calcarius lapponicus",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant lapon",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Caprimulgus europaeus",
	Group          : "Oiseaux", 
	Family         : "Caprimulgidés",
	CreavesSpecies : "Engoulevent d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Carduelis cannabina",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Linotte mélodieuse",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carduelis carduelis",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Chardonneret élégant",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carduelis chloris",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Verdier d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carduelis flammea",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Sizerin flammé",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carduelis flavirostris",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Linotte à bec jaune",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carduelis spinus",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Tarin des aulnes",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Carpodacus erythrinus",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Roselin cramoisi",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Certhia brachydactyla",
	Group          : "Oiseaux", 
	Family         : "Certhiidés",
	CreavesSpecies : "Grimperau des jardins",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Certhia familiaris",
	Group          : "Oiseaux", 
	Family         : "Certhiidés",
	CreavesSpecies : "Grimpereau des bois",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Cettia cetti",
	Group          : "Oiseaux", 
	Family         : "Cettiidés",
	CreavesSpecies : "Bouscarle de Cetti",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Cinclus cinclus",
	Group          : "Oiseaux", 
	Family         : "Cinclidés",
	CreavesSpecies : "Cincle plongeur",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Cisticola juncidis",
	Group          : "Oiseaux", 
	Family         : "Cisticolidés",
	CreavesSpecies : "Cisticole des joncs",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Coccothraustes coccothraustes",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Grosbec casse-noyaux",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Corvus corone",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Corneille noire",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Corvus frugilegus",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Corbeau freux",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Corvus monedula",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Choucas des tours",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Delichon urbica",
	Group          : "Oiseaux", 
	Family         : "Hirundinidés",
	CreavesSpecies : "Hirondelle de fenêtre",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Emberiza aureola",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant auréole",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza calandra",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant proyer",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza cia",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant fou",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza cirlus",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant zizi",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza citrinella",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant jaune",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza hortulana",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant ortolan",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza leucocephala",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant à calotte blanche",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza pusilla",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant nain",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza rustica",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant rustique",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Emberiza schoeniclus",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant des roseaux",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Alauda alpestris",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette haussecol",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Erithacus rubecula",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Rougegorge familier",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Ficedula albicollis",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Gobemouche à collier",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Ficedula hypoleuca",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Gobemouche noir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Ficedula parva",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Gobemouche nain",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Fringilla coelebs",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Pinson des arbres",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Fringilla montifringilla",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Pinson du Nord",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Galerida cristata",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Cochevis huppé",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Garrulus glandarius",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Geai des chênes",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Hippolais icterina",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Hypolaïs ictérine",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Hippolais polyglotta",
	Group          : "Oiseaux", 
	Family         : "Acrocéphalidés",
	CreavesSpecies : "Hypolaïs polyglotte",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Hirundo daurica",
	Group          : "Oiseaux", 
	Family         : "Hirundinidés",
	CreavesSpecies : "Hirondelle rousseline",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Hirundo rustica",
	Group          : "Oiseaux", 
	Family         : "Hirundinidés",
	CreavesSpecies : "Hirondelle rustique",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Lanius collurio",
	Group          : "Oiseaux", 
	Family         : "Laniidés",
	CreavesSpecies : "Pie-grièche écorcheur",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Lanius excubitor",
	Group          : "Oiseaux", 
	Family         : "Laniidés",
	CreavesSpecies : "Pie-grièche grise",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Lanius minor",
	Group          : "Oiseaux", 
	Family         : "Laniidés",
	CreavesSpecies : "Pie-grièche à poitrine rose",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Lanius senator",
	Group          : "Oiseaux", 
	Family         : "Laniidés",
	CreavesSpecies : "Pie-grièche à tête rousse",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Locustella fluviatilis",
	Group          : "Oiseaux", 
	Family         : "Locustellidés",
	CreavesSpecies : "Locustelle fluviatile",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Locustella luscinioides",
	Group          : "Oiseaux", 
	Family         : "Locustellidés",
	CreavesSpecies : "Locustelle luscinoïde",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Locustella naevia",
	Group          : "Oiseaux", 
	Family         : "Locustellidés",
	CreavesSpecies : "Locustelle tachetée",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Loxia curvirostra",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Bec-croisé des sapins",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Loxia leucoptera",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Bec-croisé bifascié",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Loxia pityopsittacus",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Bec-croisé perroquet",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Alauda alpestris",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette lulu",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Luscinia luscinia",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Rossignol progné",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Luscinia megarhynchos",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Rossignol philomèle",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Luscinia svecica",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Gorgebleue à miroir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Melanocorypha calandra",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette calandre",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Melanocorypha leucoptera",
	Group          : "Oiseaux", 
	Family         : "Alaudidés",
	CreavesSpecies : "Alouette leucoptère",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Monticola saxatilis",
	Group          : "Oiseaux", 
	Family         : "Turdidé",
	CreavesSpecies : "Monticole de roche",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Motacilla alba",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Bergeronnette grise",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Motacilla cinerea",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Bergeronnette des ruisseaux",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Motacilla flava",
	Group          : "Oiseaux", 
	Family         : "Motacillidés",
	CreavesSpecies : "Bergeronnette printanière",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Muscicapa striata",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Gobemouche gris",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Nucifraga caryocatactes",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Cassenoix moucheté",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Oenanthe hispanica",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Traquet oreillard",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Oenanthe oenanthe",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Traquet motteux",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Oriolus oriolus",
	Group          : "Oiseaux", 
	Family         : "Oriolidés",
	CreavesSpecies : "Loriot d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Panurus biarmicus",
	Group          : "Oiseaux", 
	Family         : "Panuridés",
	CreavesSpecies : "Panure à moustaches",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus ater",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange noire",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus caeruleus",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange bleue",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus cristatus",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange huppée",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus major",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange charbonnière",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus montanus",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange boréale",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Parus palustris",
	Group          : "Oiseaux", 
	Family         : "Paridés",
	CreavesSpecies : "Mésange nonnette",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Passer domesticus",
	Group          : "Oiseaux", 
	Family         : "Passeridés",
	CreavesSpecies : "Moineau domestique",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Passer montanus",
	Group          : "Oiseaux", 
	Family         : "Passeridés",
	CreavesSpecies : "Moineau friquet",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Petronia petronia",
	Group          : "Oiseaux", 
	Family         : "Passeridés",
	CreavesSpecies : "Moineau soulcie",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Phoenicurus ochruros",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Rougequeue noir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phoenicurus phoenicurus",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Rougequeue à front blanc",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus bonelli",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot de Bonelli",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus collybita",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot véloce",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus inornatus",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot à grands sourcils",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus proregulus",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot de Pallas",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus schwarzi",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot de Schwarz",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus sibilatrix",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot siffleur",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Phylloscopus trochilus",
	Group          : "Oiseaux", 
	Family         : "Phylloscopidés",
	CreavesSpecies : "Pouillot fitis",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Pica pica",
	Group          : "Oiseaux", 
	Family         : "Corvidés",
	CreavesSpecies : "Pie bavarde",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.5",
},

{
	Species        : "Plectrophenax nivalis",
	Group          : "Oiseaux", 
	Family         : "Emberizidés",
	CreavesSpecies : "Bruant des neiges",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Prunella collaris",
	Group          : "Oiseaux", 
	Family         : "Prunellidés",
	CreavesSpecies : "Accenteur alpin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Prunella modularis",
	Group          : "Oiseaux", 
	Family         : "Prunellidés",
	CreavesSpecies : "Accenteur mouchet",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Ptyonoprogne rupestris",
	Group          : "Oiseaux", 
	Family         : "Hirundinidés",
	CreavesSpecies : "Hirondelle des rochers",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Pyrrhula pyrrhula",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Bouvreuil pivoine",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Regulus ignicapillus",
	Group          : "Oiseaux", 
	Family         : "Régulidés",
	CreavesSpecies : "Roitelet à triple bandeau",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Regulus regulus",
	Group          : "Oiseaux", 
	Family         : "Régulidés",
	CreavesSpecies : "Roitelet huppé",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Remiz pendulinus",
	Group          : "Oiseaux", 
	Family         : "Rémizidés",
	CreavesSpecies : "Remiz penduline",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Riparia riparia",
	Group          : "Oiseaux", 
	Family         : "Hirundinidés",
	CreavesSpecies : "Hirondelle de rivage",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Saxicola rubetra",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Tarier des prés",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Saxicola rubicola",
	Group          : "Oiseaux", 
	Family         : "Muscicapidés",
	CreavesSpecies : "Tarier pâtre (torquatus)",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Serinus citrinella",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Venturon montagnard",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Serinus serinus",
	Group          : "Oiseaux", 
	Family         : "Fringillidés",
	CreavesSpecies : "Serin cini",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Sitta europaea",
	Group          : "Oiseaux", 
	Family         : "Sittidés",
	CreavesSpecies : "Sittelle d'Europe",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sturnus roseus",
	Group          : "Oiseaux", 
	Family         : "Sturnidés",
	CreavesSpecies : "Etourneau roselin",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sturnus vulgaris",
	Group          : "Oiseaux", 
	Family         : "Sturnidés",
	CreavesSpecies : "Etourneau sansonnet",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia atricapilla",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette à tête noire",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia borin",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette des jardins",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia cantillans",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette passerinette",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia communis",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette grisette",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia curruca",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette babillarde",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia hortensis",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette orphée",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia melanocephala",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette mélanocéphale",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia nisoria",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette épervière",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Sylvia undata",
	Group          : "Oiseaux", 
	Family         : "Sylviidés",
	CreavesSpecies : "Fauvette pitchou",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Tichodroma muraria",
	Group          : "Oiseaux", 
	Family         : "Tichodromidés",
	CreavesSpecies : "Tichodrome échelette",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Troglodytes troglodytes",
	Group          : "Oiseaux", 
	Family         : "Troglodytidés",
	CreavesSpecies : "Troglodyte mignon",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus iliacus",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive mauvis",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus merula",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Merle noir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus naumanni",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive de Nauman",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus obscurus",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive obscure",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus philomelos",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive musicienne",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus pilaris",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive litorne",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus ruficollis",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive à gorge rousse",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus torquatus",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Merle à plastron",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Turdus viscivorus",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive draine",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Zoothera dauma",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive dorée",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Zoothera sibirica",
	Group          : "Oiseaux", 
	Family         : "Turdidés",
	CreavesSpecies : "Grive de Sibérie",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Dendrocopos leucotos",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic à dos blanc",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Dendrocopos major",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic épeiche",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Dendrocopos medius",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic mar",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Dendrocopos minor",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic épeichette",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Dryocopus martius",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic noir",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Jynx torquilla",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Torcol fourmilier",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Picus canus",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic cendré",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Picus viridis",
	Group          : "Oiseaux", 
	Family         : "Picidés",
	CreavesSpecies : "Pic vert",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "1.12",
},

{
	Species        : "Accipiter gentilis",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Autour des palombes",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Accipiter nisus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Epervier d'Europe",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Aquila chrysaetos",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Aigle royal",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Aquila clanga",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Aigle criard",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Aquila pomarina",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Aigle pomarin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Buteo buteo",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Buse variable",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Buteo lagopus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Buse pattue",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Circaetus gallicus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Circaète Jean le blanc",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Circus aeruginosus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Busard des roseaux",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Circus cyaneus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Busard Saint Martin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Circus macrourus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Busard pâle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Circus pygargus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Busard cendré",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Elanus caeruleus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Elanion blanc",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Falco columbarius",
	Group          : "Oiseaux", 
	Family         : "Falconidés",
	CreavesSpecies : "Faucon émerillon",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Falco peregrinus",
	Group          : "Oiseaux", 
	Family         : "Falconidés",
	CreavesSpecies : "Faucon pèlerin",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Falco subbuteo",
	Group          : "Oiseaux", 
	Family         : "Falconidés",
	CreavesSpecies : "Faucon hobereau",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Falco tinnunculus",
	Group          : "Oiseaux", 
	Family         : "Falconidés",
	CreavesSpecies : "Faucon crécerelle",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Falco vespertinus",
	Group          : "Oiseaux", 
	Family         : "Falconidés",
	CreavesSpecies : "Faucon kobez",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Gyps fulvus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Vautour fauve",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Haliaeetus albicilla",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Pygargue à queue blanche",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Hieraaetus fasciatus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Aigle de Bonelli",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Hieraaetus pennatus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Aigle botté",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Milvus migrans",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Milan noir",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Milvus milvus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Milan royal",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Neophron percnopterus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Vautour percnoptère",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Pandion haliaetus",
	Group          : "Oiseaux", 
	Family         : "Pandionidés",
	CreavesSpecies : "Balbuzard pêcheur",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Pernis apivorus",
	Group          : "Oiseaux", 
	Family         : "Accipitridés",
	CreavesSpecies : "Bondrée apivore",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Aegolius funereus",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Chouette de Tengmalm",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Asio flammeus",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Hibou des marais",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Asio otus",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Hibou moyen duc",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Athene noctua",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Chevêche d'Athena",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Bubo bubo",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Grand-Duc d'Europe",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3",
},

{
	Species        : "Glaucidium passerinum",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Chevêchette d'Europe",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Otus scops",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Hibou petit duc scops",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Strix aluco",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Chouette hulotte",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Surnia ulula",
	Group          : "Oiseaux", 
	Family         : "Strigidés",
	CreavesSpecies : "Chouette épervière",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Tyto alba",
	Group          : "Oiseaux", 
	Family         : "Tytonidés",
	CreavesSpecies : "Effraie des clochers",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "1.37",
},

{
	Species        : "Anser anser",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Oie cendrée",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Tamias sibiricus",
	Group          : "Mammifères", 
	Family         : "Sciuridés",
	CreavesSpecies : "Tamia de Sibérie",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Dama dama",
	Group          : "Mammifères", 
	Family         : "Cervidés",
	CreavesSpecies : "Daim",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Ovis gmelini musimon x Ovis sp.",
	Group          : "Mammifères", 
	Family         : "Bovidés",
	CreavesSpecies : "Mouflon x  Mouton domestique",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Myocastor coypus",
	Group          : "Mammifères", 
	Family         : "Echimyidés",
	CreavesSpecies : "Ragondin",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Ondatra zibethicus",
	Group          : "Mammifères", 
	Family         : "Cricétidés",
	CreavesSpecies : "Rat musqué",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Rattus norvegicus",
	Group          : "Mammifères", 
	Family         : "Muridés",
	CreavesSpecies : "Rat brun",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Columba livia var. domestica",
	Group          : "Oiseaux", 
	Family         : "Colombidés",
	CreavesSpecies : "Pigeon biset féral",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Colinus virginianus",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Colin de Virginie",
	CreavesGroup   : "exotique",        
	Subside        : "0.96",
},

{
	Species        : "Lagopus lagopus",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Lagopède des saules",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Phasianus colchicus",
	Group          : "Oiseaux", 
	Family         : "Phasianidés",
	CreavesSpecies : "Faisan de Colchide",
	CreavesGroup   : "Autres oiseaux et chauves-souris",        
	Subside        : "0.96",
},

{
	Species        : "Aix galericulata",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Canard mandarin",
	CreavesGroup   : "exotique",        
	Subside        : "",
},

{
	Species        : "Alopochen aegyptiacus",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Ouette d'Egypte",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Branta canadensis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Bernache du Canada",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Cygnus olor",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Cygne tuberculé",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Oxyura jamaicensis",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Erismature rousse",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Psittacula krameri",
	Group          : "Oiseaux", 
	Family         : "Psittacidés",
	CreavesSpecies : "Perruche à collier",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Felis catus",
	Group          : "Mammifères", 
	Family         : "Félidés",
	CreavesSpecies : "Chat haret",
	CreavesGroup   : "domestique",        
	Subside        : "",
},

{
	Species        : "Genetta genetta",
	Group          : "Mammifères", 
	Family         : "Viverridés",
	CreavesSpecies : "Genette",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Mustela vison (Neovison vison)",
	Group          : "Mammifères", 
	Family         : "Mustélidés",
	CreavesSpecies : "Vison américain",
	CreavesGroup   : "invasif",        
	Subside        : "",
},

{
	Species        : "Procyon lotor",
	Group          : "Mammifères", 
	Family         : "Procyonidés",
	CreavesSpecies : "Raton laveur",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Nyctereutes procyonoides",
	Group          : "Mammifères", 
	Family         : "Canidés",
	CreavesSpecies : "Chien viverrin",
	CreavesGroup   : "invasif préocupant",        
	Subside        : "",
},

{
	Species        : "Castor fiber",
	Group          : "Mammifères", 
	Family         : "Castoridés",
	CreavesSpecies : "Castor d'Europe",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Lynx lynx",
	Group          : "Mammifères", 
	Family         : "Félidés",
	CreavesSpecies : "Lynx boréal",
	CreavesGroup   : "Mammifères non volants",        
	Subside        : "",
},

{
	Species        : "Anas querquedula",
	Group          : "Oiseaux", 
	Family         : "Anatidés",
	CreavesSpecies : "Sarcelle d'été",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

{
	Species        : "Larus michahellis",
	Group          : "Oiseaux", 
	Family         : "Laridés",
	CreavesSpecies : "Goéland leucophée",
	CreavesGroup   : "Rapaces, oiseaux d’eau, échassiers ou limicoles",        
	Subside        : "3.21",
},

	}
	
	cnt, err := models.DB.Q().Count(&models.Species{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in species - skipping\n", cnt)
		return nil
	}


	return models.DB.Transaction(func(con *pop.Connection) error {
		for _, t := range ts {
			if exists, err := con.Where("Species = ?", t.Species).Exists(&models.Species{}); err != nil {
				return err
			} else if !exists {
				d := &models.Species{
					Species:        t.Species,
					Group:          t.Group,
					Family:         t.Family,
					CreavesSpecies: t.CreavesSpecies,
					CreavesGroup:   t.CreavesGroup,
				}
				if len(t.Subside) > 0 {
					dsf, err := strconv.ParseFloat(t.Subside, 64)
					if err == nil {
						d.Subside = nulls.NewFloat64(dsf)
					} else {
						fmt.Printf("Error parsing subside %s: %v", t.Subside, err)
					}
				}
				if err := con.Create(d); err != nil {
					return err
				}
			} else {
				fmt.Printf("Species %s already exists\n", t.Species)
			}
		}
		return nil
	})
}
