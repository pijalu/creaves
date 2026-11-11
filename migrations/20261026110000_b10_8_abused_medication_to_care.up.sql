UPDATE care_animal_plans cap
SET cap.action_kind = 'care',
    cap.name = REPLACE(cap.name, 'Traitement — ', 'Soin — '),
    cap.action_payload = JSON_SET(
        JSON_REMOVE(cap.action_payload, '$.alert_on', '$.alert_follow_up_hours'),
        '$.caretype_id', (SELECT id FROM (SELECT id FROM caretypes WHERE name = 'Soin' ORDER BY created_at LIMIT 1) soin),
        '$.instructions', COALESCE(JSON_UNQUOTE(JSON_EXTRACT(cap.action_payload, '$.note')), ''),
        '$.note', JSON_UNQUOTE(JSON_EXTRACT(cap.action_payload, '$.prompt')))
WHERE cap.created_by IS NULL
  AND cap.active = 1
  AND cap.action_kind = 'observation'
  AND (cap.name LIKE 'Traitement — %' OR cap.name LIKE 'Soin de plaie (conversion)%')
  AND cap.action_payload IS NOT NULL
  AND JSON_VALID(cap.action_payload)
  AND JSON_EXTRACT(cap.action_payload, '$.prompt') IS NOT NULL
  AND JSON_TYPE(JSON_EXTRACT(cap.action_payload, '$.prompt')) = 'STRING'
  AND EXISTS (SELECT 1 FROM caretypes WHERE name = 'Soin');
