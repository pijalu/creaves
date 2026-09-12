-- DATA MIGRATION (reviewed) -- do not fold into the code-assignment schema
-- migration. Normalizes outtaketype display names, descriptions and the
-- dead/error/rating flags to the canonical values, keyed by the stable OT
-- codes assigned in 20260912120000_add_outtaketype_code. Kept separate so the
-- destructive rename/flag rewrite can be reviewed on its own.
--
-- Down is intentionally lossy: OT1-OT4 names are unchanged by the up step and
-- prior per-center flag/description values cannot be reconstructed. The
-- create_outtaketype grift re-applies the canonical flags on the next startup.
UPDATE outtaketypes SET name = CASE code WHEN 'OT1' THEN 'Relacher' WHEN 'OT2' THEN 'DCD' WHEN 'OT3' THEN 'Euthanasier' WHEN 'OT4' THEN 'Transferer' WHEN 'OT5' THEN 'Mort à l''arrivée avant l''encodage' WHEN 'OT6' THEN 'Adoption' WHEN 'OT7' THEN 'Doublon' ELSE name END, description = CASE code WHEN 'OT1' THEN 'Animal réhabilité et remis en liberté dans son milieu naturel.' WHEN 'OT2' THEN 'Animal décédé naturellement durant la prise en charge.' WHEN 'OT3' THEN 'Animal euthanasié en raison de lésions ou d’un état incompatible avec une remise en liberté.' WHEN 'OT4' THEN 'Transfert de l''animal vers un: refuge, CREAVES, VOC, ZOO, ...' WHEN 'OT5' THEN 'Animal arrivé décédé avant l''encodage ou la prise en charge.' WHEN 'OT6' THEN 'Animal placé en captivité autorisée car espèce non indigène.' WHEN 'OT7' THEN 'Fiche créée en double pour le même animal.' ELSE description END WHERE code IN ('OT1','OT2','OT3','OT4','OT5','OT6','OT7');
UPDATE outtaketypes SET dead = CASE code WHEN 'OT2' THEN 1 WHEN 'OT3' THEN 1 WHEN 'OT5' THEN 1 ELSE 0 END, error = CASE code WHEN 'OT7' THEN 1 ELSE 0 END, rating = CASE code WHEN 'OT1' THEN 1 WHEN 'OT2' THEN -1 WHEN 'OT3' THEN -1 WHEN 'OT4' THEN 1 WHEN 'OT5' THEN -1 WHEN 'OT6' THEN 0 WHEN 'OT7' THEN -1 ELSE rating END WHERE code IN ('OT1','OT2','OT3','OT4','OT5','OT6','OT7');
