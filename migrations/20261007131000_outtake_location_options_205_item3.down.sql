-- Revert #205 item 3 outtake_location_options integration.
-- Removes the 33 rows that did not exist before and restores the two renamed
-- pre-existing rows; re-inserts their generic en-US/de/nl translations.

DELETE FROM translations
WHERE table_name = 'outtake_location_options'
  AND record_id IN ('7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03');

INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01', 'name', 'en-US', 'Wildlife rescue center', NOW(), NOW()),
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01', 'name', 'de', 'Wildtierauffangstation', NOW(), NOW()),
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01', 'name', 'nl', 'Wildopvangcentrum', NOW(), NOW()),
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03', 'name', 'en-US', 'VOC', NOW(), NOW()),
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03', 'name', 'de', 'VOC', NOW(), NOW()),
(UUID(), 'outtake_location_options', '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03', 'name', 'nl', 'VOC', NOW(), NOW());

UPDATE outtake_location_options SET name = 'Creaves' WHERE id = '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01';
UPDATE outtake_location_options SET name = 'VOC' WHERE id = '7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03';

DELETE FROM outtake_location_options WHERE id IN (
'08d1e0ad-43ea-4d49-9bae-c40159596bdd',
'0dae39b9-e7a2-4c11-8494-a0a228a11fe5',
'128daff1-203b-418c-b712-5bd72c21d3f1',
'1ad44496-dcbf-42f5-ab80-cc8d14bf24c0',
'1ea2b4bb-db77-4a9d-b15a-1900d80a7675',
'20fbf1d2-100b-4bfa-bac4-4c4b1424148c',
'2d06d092-2e42-4f63-b57e-05f721800f20',
'2f813174-ca04-4669-bfc7-cf390757ec47',
'3d557823-fe08-4757-8745-7a70da6b6b45',
'3fb771db-14ae-4e37-bf63-2c145fbfcf8c',
'5813dda7-ce51-46f8-8b9f-df9b6689028a',
'5c32e283-34fc-4fed-9a1b-d47861f4efcb',
'5ec4bede-d886-4fff-a0d5-6d8b8c06e54b',
'64696459-79f6-4c2e-bbb9-0a5c7ea568a6',
'78a9ef05-84dd-4934-9436-c21decf68df7',
'858aec3e-a02e-4cd5-a325-1610d61e1508',
'99d36fe4-6ab0-45dd-b0ee-fe0e07baa6c1',
'ad8c55f7-2f27-4b3b-90a5-7cb5e449e23e',
'b5cd01ad-48f5-486b-ab19-4e4b2c67e6a4',
'bb096932-1d04-41ec-8351-2598e5c76a6f',
'bd573bf6-8beb-465f-b517-40e5b6091122',
'bf9ed094-9a0f-4834-9655-30df16bc65f3',
'c87b56f2-a1e6-4c20-bfd4-a27796047a5d',
'c91fd21d-c64a-4937-a3c1-051c7144c313',
'ca08b33f-44c1-4e59-9b77-11e64756416e',
'd062f8c9-7429-4bb1-bc11-0378829b9b51',
'd1349eae-80e4-41a7-a928-389fa84eea5e',
'd40853ac-44b1-4f78-a31d-6eac4f407122',
'd558f8ae-c6a7-4e14-a140-d001381fea05',
'dfd276e2-36f5-4939-94eb-fd2956503a21',
'e1866cb3-0562-4b87-8bb6-3a3af0f71021',
'e224d609-71c4-41e7-b26b-5c5710c9d40b',
'ec5043c8-93ec-43ba-acbb-325a40ba24f8');
