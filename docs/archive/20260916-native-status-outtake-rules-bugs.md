# Native status
A new ADMIN CRUD for native statuses should be added - the table should be translated as well.
the ADMIN CRUD should *only* be available to maintainers

# Species view/edit
Species table should be fully viewable and editable.
Native status should be a list - using native_statuses values.

# Outtake: additional rules (outtakes/new)
Based on this table (matching by `id_N` - focus on ID n_s interdit + outtake_location)
```
id_N,name,description,def,created_at,updated_at,dead,rating,discoverer_news,error,ID n_s interdit,outtake_location
OT1,Relacher,animal is freed,0,2021-04-05 18:57:33,2021-10-23 22:36:20,0,1,"Nous sommes heureux de vous informer que, suite à un parcours de soins en notre centre, nous avons pu relâcher l’animal que vous nous aviez confié.",0,"NS3, NS2, NS4",0000_Village
OT2,DCD,animal died,1,2021-04-05 18:57:33,2021-04-11 17:33:57,1,-1,"Malheureusement, et ce malgré nos bons soins, nous n’avons pas pu sauver l’animal que vous êtes venus nous déposer.",0,,AUCUN
OT3,Euthanasier,,0,2021-10-23 22:36:05,2021-10-23 22:36:05,1,-1,"Malheureusement, et ce malgré nos bons soins, nous n’avons pas pu sauver l’animal que vous êtes venus nous déposer.",0,,AUCUN
OT4,Transferer,animal is transfered,0,2021-04-05 18:57:33,2021-10-23 22:36:35,0,1,NULL,0,NS3,"sur base de la BDD (CREAVES, REFUGE, VOC, ZOO)"
OT5,Mort à l'arrivée avant l'encodage,Mort avant l'encodage de l'animal,0,2023-05-02 10:34:14,2024-04-05 19:33:42,1,0,,0,,AUCUN
OT6,Adoption,Adoption d'un animal non sauvage.,0,2022-03-30 08:53:45,2024-12-18 09:26:19,0,0,l’animal fut adopter,0,"NS1, NS3","sur base de la BDD (CREAVES, REFUGE, VOC, ZOO)"
OT7,Doublon,,0,2024-05-23 08:07:22,2024-05-23 08:07:22,0,0,,1,,AUCUN%
```

=> Outtake selection  should be filter out based on the species 'native_status' - eg: OT1 should not be possible for species native_status in ("NS2", "NS3", "NS4")

=> Lieu selection should only be available for OT1

OT4 + OT6: there should be a list of specific options (Creaves / Refuge / VOC / Zoo) - you can create a dedicated table or add the list of option to a new outtake - the option *must* be translated

---
Archived resolved entries from bugs.md on 2026-09-16:
- Native status admin CRUD -> creaves commit a904570 (+ eee98a3), plan docs in creaves/docs/
- Species view/edit + native status list -> creaves commit eee98a3, plan creaves/docs/species-native-status-list.md
- Outtake additional rules -> plan creaves/docs/outtake-native-status-rules.md
