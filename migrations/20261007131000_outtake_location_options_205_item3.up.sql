-- #205 item 3: integrate the updated outtake_location_options reference table
-- (37 centers). The 4 pre-existing synthetic UUIDs are kept by the ticket CSV
-- (two of them renamed to proper-noun center names); the other 33 rows are new.
-- Outtakes store the resolved location name as plain text, so nothing needs
-- re-keying. INSERT ... ON DUPLICATE KEY UPDATE force-corrects the canonical
-- name of rows that were renamed upstream.

INSERT INTO outtake_location_options (id, name, created_at, updated_at) VALUES
('08d1e0ad-43ea-4d49-9bae-c40159596bdd','CREAVES de Hotton','2026-09-26 11:20:30','2026-09-26 11:20:30'),
('0dae39b9-e7a2-4c11-8494-a0a228a11fe5','CREAVES des Terrils','2026-09-26 11:19:14','2026-09-26 11:19:14'),
('128daff1-203b-418c-b712-5bd72c21d3f1','Refuge Carapace','2026-09-26 11:33:06','2026-09-26 11:33:06'),
('1ad44496-dcbf-42f5-ab80-cc8d14bf24c0','CREAVES de Namur','2026-09-24 22:19:36','2026-09-24 22:19:36'),
('1ea2b4bb-db77-4a9d-b15a-1900d80a7675','CREAVES de Ransart','2026-09-26 11:21:40','2026-09-26 11:21:40'),
('20fbf1d2-100b-4bfa-bac4-4c4b1424148c','CREAVES « Le Martinet »','2026-09-26 11:19:25','2026-09-26 11:19:25'),
('2d06d092-2e42-4f63-b57e-05f721800f20','CREAVES de Sprimont','2026-09-26 11:19:36','2026-09-26 11:19:36'),
('2f813174-ca04-4669-bfc7-cf390757ec47','CREAVES de Virelles','2026-09-26 11:20:58','2026-09-26 11:20:58'),
('3d557823-fe08-4757-8745-7a70da6b6b45','VOC Wilde Dieren in Nood','2026-09-26 11:30:49','2026-09-26 11:30:49'),
('3fb771db-14ae-4e37-bf63-2c145fbfcf8c','VOC Egel Hulpcentrum Avalon','2026-09-26 11:31:44','2026-09-26 11:31:44'),
('5813dda7-ce51-46f8-8b9f-df9b6689028a','CREAVES de St Hubert','2026-09-26 11:27:39','2026-09-26 11:27:39'),
('5c32e283-34fc-4fed-9a1b-d47861f4efcb','CREAVES de BIrds Bay','2026-09-24 22:19:21','2026-09-24 22:19:21'),
('5ec4bede-d886-4fff-a0d5-6d8b8c06e54b','Centre de Soins Bruxelles - LRBPO','2026-09-26 11:21:52','2026-09-26 11:21:52'),
('64696459-79f6-4c2e-bbb9-0a5c7ea568a6','VOC Malderen','2026-09-26 11:31:06','2026-09-26 11:31:06'),
('78a9ef05-84dd-4934-9436-c21decf68df7','CREAVES « l’hermitage à Thimister »','2026-09-26 11:19:49','2026-09-26 11:19:49'),
('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01','CREAVES de Pairai Daiza','2026-09-18 08:15:55','2026-09-24 22:28:16'),
('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b02','Refuge','2026-09-18 08:15:55','2026-09-18 08:15:55'),
('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03','VOC Oostende','2026-09-18 08:15:55','2026-09-26 11:30:18'),
('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b04','Zoo','2026-09-18 08:15:55','2026-09-18 08:15:55'),
('858aec3e-a02e-4cd5-a325-1610d61e1508','refuge Opale','2026-09-26 11:32:41','2026-09-26 11:32:41'),
('99d36fe4-6ab0-45dd-b0ee-fe0e07baa6c1','CREAVES « le Chalet des Hirchons »','2026-09-26 11:21:10','2026-09-26 11:21:10'),
('ad8c55f7-2f27-4b3b-90a5-7cb5e449e23e','VOC Natuurhulpcentrum','2026-09-26 11:31:13','2026-09-26 11:32:15'),
('b5cd01ad-48f5-486b-ab19-4e4b2c67e6a4','CREAVES de Dour','2026-09-26 11:21:19','2026-09-26 11:21:19'),
('bb096932-1d04-41ec-8351-2598e5c76a6f','SOS Wilde Dieren','2026-09-26 11:30:42','2026-09-26 11:30:42'),
('bd573bf6-8beb-465f-b517-40e5b6091122','VOC Merelbeke','2026-09-26 11:30:34','2026-09-26 11:30:34'),
('bf9ed094-9a0f-4834-9655-30df16bc65f3','Refuge L''Arche','2026-09-26 11:32:49','2026-09-26 11:32:49'),
('c87b56f2-a1e6-4c20-bfd4-a27796047a5d','Zoo Pairi-Daiza','2026-09-24 22:18:20','2026-09-24 22:18:20'),
('c91fd21d-c64a-4937-a3c1-051c7144c313','CREAVES de Murringen','2026-09-26 11:20:02','2026-09-26 11:20:02'),
('ca08b33f-44c1-4e59-9b77-11e64756416e','CREAVES  « La Tanière des Fagnes »','2026-09-26 11:20:21','2026-09-26 11:20:21'),
('d062f8c9-7429-4bb1-bc11-0378829b9b51','VOC Beernem','2026-09-26 11:30:27','2026-09-26 11:30:27'),
('d1349eae-80e4-41a7-a928-389fa84eea5e','CREAVES de Frasnes-lez-Anvaing','2026-09-26 11:21:28','2026-09-26 11:21:28'),
('d40853ac-44b1-4f78-a31d-6eac4f407122','VOC Neteland','2026-09-26 11:30:57','2026-09-26 11:30:57'),
('d558f8ae-c6a7-4e14-a140-d001381fea05','CREAVES d’Herbeumont','2026-09-26 11:20:39','2026-09-26 11:20:39'),
('dfd276e2-36f5-4939-94eb-fd2956503a21','Zoo Domaine des Grottes de Han','2026-09-26 11:38:04','2026-09-26 11:38:04'),
('e1866cb3-0562-4b87-8bb6-3a3af0f71021','CREAVES L''Arche de Lorraine','2026-09-26 11:28:40','2026-09-26 11:28:40'),
('e224d609-71c4-41e7-b26b-5c5710c9d40b','CREAVES de Perwez','2026-09-24 19:39:13','2026-09-24 19:39:13'),
('ec5043c8-93ec-43ba-acbb-325a40ba24f8','CREAVES d''Andenne','2026-09-24 22:18:45','2026-09-24 22:18:45')
ON DUPLICATE KEY UPDATE name = VALUES(name), updated_at = NOW();

-- The two renamed rows are proper nouns now: their generic en-US/de/nl
-- translations ("Wildlife rescue center" / "VOC") are stale and must not
-- shadow the canonical names. All other new rows are proper nouns with no
-- translations (they display the canonical name in every locale).
DELETE FROM translations
WHERE table_name = 'outtake_location_options'
  AND record_id IN ('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03');
