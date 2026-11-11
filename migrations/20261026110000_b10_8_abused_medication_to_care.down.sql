UPDATE care_animal_plans cap
SET cap.action_kind = 'observation',
    cap.name = REPLACE(cap.name, 'Soin — ', 'Traitement — '),
    cap.action_payload = JSON_SET(
        JSON_REMOVE(cap.action_payload, '$.caretype_id'),
        '$.prompt', JSON_UNQUOTE(JSON_EXTRACT(cap.action_payload, '$.note')),
        '$.note', COALESCE(JSON_UNQUOTE(JSON_EXTRACT(cap.action_payload, '$.instructions')), ''))
WHERE cap.created_by IS NULL
  AND cap.action_kind = 'care'
  AND (cap.name LIKE 'Soin — %' OR cap.name LIKE 'Soin de plaie (conversion)%')
  AND cap.action_payload IS NOT NULL
  AND JSON_VALID(cap.action_payload)
  AND JSON_EXTRACT(cap.action_payload, '$.caretype_id') IS NOT NULL;
