-- Reverse of 20261007100000_derived_matchers.up.sql: restore the suffixed
-- seed names and their localized display names, then drop the flag.

UPDATE care_matchers SET name = 'Hérisson bébé < 300 g (dérivé SM1)' WHERE name = 'Hérisson bébé < 300 g';
UPDATE care_matchers SET name = 'Colombidés bébé (dérivé SM4+SM11)' WHERE name = 'Colombidés bébé';
UPDATE care_matchers SET name = 'Canidés bébé (dérivé SM6+SM11)' WHERE name = 'Canidés bébé';
UPDATE care_matchers SET name = 'Hérisson bébé ou juvénile (dérivé SM1 OU SM2)' WHERE name = 'Hérisson bébé ou juvénile';

UPDATE translations t
  JOIN care_matchers m ON m.id = t.record_id
  SET t.value = CONCAT(t.value, CASE m.name
    WHEN 'Hérisson bébé < 300 g (dérivé SM1)' THEN CASE t.locale
      WHEN 'de' THEN ' (abgeleitet SM1)'
      WHEN 'nl' THEN ' (afgeleid SM1)'
      ELSE ' (derived SM1)' END
    WHEN 'Colombidés bébé (dérivé SM4+SM11)' THEN CASE t.locale
      WHEN 'de' THEN ' (abgeleitet SM4+SM11)'
      WHEN 'nl' THEN ' (afgeleid SM4+SM11)'
      ELSE ' (derived SM4+SM11)' END
    WHEN 'Canidés bébé (dérivé SM6+SM11)' THEN CASE t.locale
      WHEN 'de' THEN ' (abgeleitet SM6+SM11)'
      WHEN 'nl' THEN ' (afgeleid SM6+SM11)'
      ELSE ' (derived SM6+SM11)' END
    WHEN 'Hérisson bébé ou juvénile (dérivé SM1 OU SM2)' THEN CASE t.locale
      WHEN 'de' THEN ' (abgeleitet SM1 ODER SM2)'
      WHEN 'nl' THEN ' (afgeleid SM1 OF SM2)'
      ELSE ' (derived SM1 OR SM2)' END
    ELSE '' END)
  WHERE t.table_name = 'care_matchers' AND m.derived = 1;

ALTER TABLE care_matchers DROP COLUMN derived;
